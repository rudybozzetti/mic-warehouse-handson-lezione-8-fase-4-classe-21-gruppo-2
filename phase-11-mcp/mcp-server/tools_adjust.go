package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ▸ Flex task — adjust_inventory (skip if behind schedule)
//
// Same pattern a third time, on the inventory endpoint. Nothing later
// depends on this tool: if the hour is running out, stop here and go to the
// restitution questions in the README. The specification is
// tools_adjust_test.go (ships red).

// AdjustInventoryInput is the tool's input schema.
//
// TODO Phase 11 Flex a — write the jsonschema descriptions. Delta is signed:
// say so, with an example in each direction.
type AdjustInventoryInput struct {
	ArticleID    string `json:"article_id" jsonschema:"required: the id of the article to adjust, as returned by list_articles or create_article"`
	LocationCode string `json:"location_code" jsonschema:"required: the warehouse location code holding the stock to adjust, e.g. MAIN"`
	Delta        int32  `json:"delta" jsonschema:"required: signed change to apply to the current quantity — positive to add stock (5 means +5 units received), negative to remove it (-5 means 5 units consumed, sold or lost)"`
	Reason       string `json:"reason" jsonschema:"required: a short human-readable reason for the adjustment, e.g. 'damaged goods' or 'stock recount'"`
}

// AdjustInventoryOutput is the tool's output schema.
type AdjustInventoryOutput struct {
	Article Article `json:"article" jsonschema:"the article after the adjustment, with updated stock levels"`
}

func registerAdjustInventory(s *mcp.Server, bc *BCClient) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "adjust_inventory",
		Description: "Change the stock quantity of an article at one warehouse location. " +
			"This is a permanent write, not a query: use it only when the user explicitly " +
			"wants to record stock moving in or out (receiving, selling, damage, recount), " +
			"not to check current stock (use get_article for that). Requires article_id, " +
			"location_code, delta and reason. delta is signed: a positive value adds stock, " +
			"a negative value removes it — it is a change, never the resulting total.",
	}, adjustInventoryHandler(bc))
}

func adjustInventoryHandler(bc *BCClient) mcp.ToolHandlerFor[AdjustInventoryInput, AdjustInventoryOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in AdjustInventoryInput) (*mcp.CallToolResult, AdjustInventoryOutput, error) {
		if strings.TrimSpace(in.ArticleID) == "" {
			return nil, AdjustInventoryOutput{}, errors.New("article_id is required to adjust inventory")
		}
		if strings.TrimSpace(in.LocationCode) == "" {
			return nil, AdjustInventoryOutput{}, errors.New("location_code is required to adjust inventory")
		}

		body := struct {
			LocationCode string `json:"location_code"`
			Delta        int32  `json:"delta"`
			Reason       string `json:"reason"`
		}{
			LocationCode: in.LocationCode,
			Delta:        in.Delta,
			Reason:       in.Reason,
		}

		var article Article
		err := bc.DoJSON(ctx, http.MethodPost, "/articles/"+url.PathEscape(in.ArticleID)+"/inventory/adjust", body, &article)

		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, AdjustInventoryOutput{}, fmt.Errorf("no article with id %q exists; list_articles shows the ids that do", in.ArticleID)
		}
		if err != nil {
			return nil, AdjustInventoryOutput{}, err
		}
		return nil, AdjustInventoryOutput{Article: article}, nil
	}
}
