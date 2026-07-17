package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ▸ Task 1 — list_articles
//
// The BC endpoint GET /articles returns EVERY article and takes no filters:
// fine for a program, hostile for a conversation. Your tool shapes the
// surface: filter and cap HERE, so the agent gets what it asked for and
// nothing more. The specification is tools_list_test.go (ships red).

// ListArticlesInput is the tool's input schema.
//
// TODO Phase 11 Task 1a — write the two jsonschema descriptions. Each one
// must say what the field means AND what happens when it is omitted; the
// agent decides how to call you based only on this text.
type ListArticlesInput struct {
	Query string `json:"query,omitempty" jsonschema:"optional case-insensitive substring to match against the article SKU or name; omit to return every article"`
	Limit int    `json:"limit,omitempty" jsonschema:"optional maximum number of articles to return; omit or set to 0 to use the default of 20"`
}

// ListArticlesOutput is the tool's output schema.
type ListArticlesOutput struct {
	Count    int       `json:"count" jsonschema:"number of articles returned, after filter and limit"`
	Articles []Article `json:"articles"`
}

func registerListArticles(s *mcp.Server, bc *BCClient) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_articles",
		Description: "Browse or search the warehouse catalogue. Returns articles whose SKU " +
			"or name contains the optional query text (case-insensitive), capped at limit " +
			"(default 20). Use this to find articles by name or SKU, or to see what is in " +
			"stock; once you know an article's id, use get_article for its full detail " +
			"including per-location inventory.",
	}, listArticlesHandler(bc))
}

func listArticlesHandler(bc *BCClient) mcp.ToolHandlerFor[ListArticlesInput, ListArticlesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in ListArticlesInput) (*mcp.CallToolResult, ListArticlesOutput, error) {
		var articles []Article
		if err := bc.DoJSON(ctx, http.MethodGet, "/articles", nil, &articles); err != nil {
			return nil, ListArticlesOutput{}, err
		}

		if in.Query != "" {
			query := strings.ToLower(in.Query)
			filtered := articles[:0]
			for _, a := range articles {
				if strings.Contains(strings.ToLower(a.SKU), query) || strings.Contains(strings.ToLower(a.Name), query) {
					filtered = append(filtered, a)
				}
			}
			articles = filtered
		}

		limit := in.Limit
		if limit == 0 {
			limit = 20
		}
		if len(articles) > limit {
			articles = articles[:limit]
		}

		return nil, ListArticlesOutput{Count: len(articles), Articles: articles}, nil
	}
}
