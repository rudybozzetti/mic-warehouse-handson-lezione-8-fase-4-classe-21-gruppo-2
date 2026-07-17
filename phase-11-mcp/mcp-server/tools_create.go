package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ▸ Task 2 — create_article
//
// This tool WRITES to the warehouse. The BC does not treat the agent
// specially: the request goes through the same auth middleware and the same
// policy you wrote in Phase 07. It succeeds only because the MCP server's
// identity (svc-warehouse-agent) is in the article_creators set; look at
// ../policies/warehouse.rego — that one line is the whole permission.
//
// A write tool raises the bar on the description: the agent must understand
// from your text that this has a side effect, when it is appropriate, and
// exactly which fields are required. The specification is
// tools_create_test.go (ships red).

// CreateArticleInput is the tool's input schema. The BC mints the article id
// at persistence: the agent never supplies one.
//
// TODO Phase 11 Task 2a — write the jsonschema descriptions. Mark what is
// required and what is optional with its default; be precise about
// price_cents (integer cents, not a decimal).
type CreateArticleInput struct {
	SKU         string `json:"sku" jsonschema:"required: the stock keeping unit code for the new article, e.g. SKU-WIDGET-01"`
	Name        string `json:"name" jsonschema:"required: the human-readable name of the article"`
	Description string `json:"description,omitempty" jsonschema:"optional free-text description; omit if not given"`
	PriceCents  int64  `json:"price_cents" jsonschema:"required: the price in integer euro cents, never a decimal — 12.99 EUR must be sent as 1299"`
	Currency    string `json:"currency,omitempty" jsonschema:"optional ISO 4217 currency code, e.g. EUR; omit to default to EUR"`
}

// CreateArticleOutput is the tool's output schema.
type CreateArticleOutput struct {
	Article Article `json:"article" jsonschema:"the persisted article, with the id minted by the BC"`
}

func registerCreateArticle(s *mcp.Server, bc *BCClient) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "create_article",
		Description: "Create a new article in the warehouse. This is a permanent write, " +
			"not a query: use it only when the user explicitly wants a new article added, " +
			"not to check whether one exists (use list_articles or get_article for that). " +
			"Requires sku, name and price_cents (integer cents, e.g. 1299 for 12.99); " +
			"description and currency are optional, currency defaults to EUR. The id is " +
			"minted by the warehouse and returned in the result; never invent one.",
	}, createArticleHandler(bc))
}

func createArticleHandler(bc *BCClient) mcp.ToolHandlerFor[CreateArticleInput, CreateArticleOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in CreateArticleInput) (*mcp.CallToolResult, CreateArticleOutput, error) {
		if strings.TrimSpace(in.SKU) == "" {
			return nil, CreateArticleOutput{}, errors.New("sku is required to create an article")
		}
		if strings.TrimSpace(in.Name) == "" {
			return nil, CreateArticleOutput{}, errors.New("name is required to create an article")
		}

		if in.Currency == "" {
			in.Currency = "EUR"
		}

		var article Article
		err := bc.DoJSON(ctx, http.MethodPost, "/articles", in, &article)

		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
			return nil, CreateArticleOutput{}, fmt.Errorf("the warehouse policy denies this identity permission to create articles: %s", apiErr.Body)
		}
		if err != nil {
			return nil, CreateArticleOutput{}, err
		}
		return nil, CreateArticleOutput{Article: article}, nil
	}
}
