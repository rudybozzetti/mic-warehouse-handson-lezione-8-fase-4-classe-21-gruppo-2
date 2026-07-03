package usecases

import (
	"context"
	"fmt"

	"warehouse.local/core/dispatcher"
	"warehouse.local/core/entities"
	"warehouse.local/core/events"
	"warehouse.local/core/interfaces"
)

// ChangeArticlePriceInput carries the primitive request shape for a price
// change application workflow.
type ChangeArticlePriceInput struct {
	ArticleID     string
	NewPriceCents int64
	Currency      string
}

// ChangeArticlePriceOutput exposes the mutated aggregate to the caller.
type ChangeArticlePriceOutput struct {
	Article *entities.Article
}

// ChangeArticlePriceUseCase loads an Article, asks the aggregate to change its
// price, persists the result, and dispatches the canonical price-changed event.
//
// This file is intentionally a starter. Complete Execute using the same
// workflow pattern already used by CreateArticleUseCase (the given example).
type ChangeArticlePriceUseCase struct {
	repo       interfaces.ArticleRepository
	dispatcher dispatcher.EventDispatcher
}

func NewChangeArticlePriceUseCase(repo interfaces.ArticleRepository, d dispatcher.EventDispatcher) *ChangeArticlePriceUseCase {
	return &ChangeArticlePriceUseCase{repo: repo, dispatcher: d}
}

func (uc *ChangeArticlePriceUseCase) Execute(ctx context.Context, in ChangeArticlePriceInput) (*ChangeArticlePriceOutput, error) {
	newPrice, err := entities.NewMoney(in.NewPriceCents, in.Currency)
	if err != nil {
		return nil, fmt.Errorf("ChangeArticlePrice: %w", err)
	}
	a, err := uc.repo.FindByID(ctx, in.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("ChangeArticlePrice: %w", err)
	}
	oldPrice := a.Price
	if err := a.ChangePrice(*newPrice); err != nil {
		return nil, fmt.Errorf("ChangeArticlePrice: %w", err)
	}
	if oldPrice.Equals(*newPrice) {
		return &ChangeArticlePriceOutput{Article: a}, nil
	}

	canonical := events.ArticlePriceChanged{
		ArticleID:     a.ID,
		OldPriceCents: oldPrice.AmountCents,
		NewPriceCents: a.Price.AmountCents,
		Currency:      a.Price.Currency,
		At:            a.UpdatedAt,
	}

	if err := uc.repo.Save(ctx, a); err != nil {
		return nil, fmt.Errorf("ChangeArticlePrice: save: %w", err)
	}
	if err := uc.dispatcher.Dispatch(ctx, []events.DomainEvent{canonical}); err != nil {
		return nil, fmt.Errorf("ChangeArticlePrice: dispatch: %w", err)
	}
	a.ClearPendingEvents()

	return &ChangeArticlePriceOutput{Article: a}, nil
}
