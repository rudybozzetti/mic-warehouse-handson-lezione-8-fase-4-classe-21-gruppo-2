package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"warehouse.local/core/entities"
	"warehouse.local/core/interfaces"
)

// legacyDefaultCurrency is assumed for every legacy row, since legacy_db has
// no currency column.
const legacyDefaultCurrency = "EUR"

// LegacyMySQLArticleRepository implements interfaces.ArticleRepository against
// the simplified legacy MySQL schema (legacy-init.sql).
//
// This file is intentionally a starter. In Task 1 you will complete it with AI
// support so the dual-write decorator can write to:
//
//   - legacy_db.articles       (price DECIMAL(10,2), no currency column)
//   - warehouse_db.articles    (price_cents BIGINT + currency CHAR(3))
//
// The important design point: this adapter is an Anti-Corruption Layer. It
// translates the legacy schema into the Warehouse domain model without making
// DualWriteArticleRepository or entities.Article know legacy details.
type LegacyMySQLArticleRepository struct {
	db *sql.DB
}

func NewLegacyMySQLArticleRepository(db *sql.DB) *LegacyMySQLArticleRepository {
	return &LegacyMySQLArticleRepository{db: db}
}

// compile-time interface check
var _ interfaces.ArticleRepository = (*LegacyMySQLArticleRepository)(nil)

func (r *LegacyMySQLArticleRepository) Save(ctx context.Context, a *entities.Article) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	const upsert = `
		INSERT INTO articles (id, sku, name, description, price, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  sku = VALUES(sku),
		  name = VALUES(name),
		  description = VALUES(description),
		  price = VALUES(price),
		  updated_at = VALUES(updated_at)
	`
	_, err := r.db.ExecContext(ctx, upsert,
		a.ID, a.SKU.Code, a.Name, a.Description,
		centsToDecimal(a.Price.AmountCents),
		a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *LegacyMySQLArticleRepository) FindByID(ctx context.Context, id string) (*entities.Article, error) {
	const q = `SELECT id, sku, name, description, price, created_at, updated_at FROM articles WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	return scanLegacyArticle(row)
}

func (r *LegacyMySQLArticleRepository) FindBySKU(ctx context.Context, skuCode string) (*entities.Article, error) {
	const q = `SELECT id, sku, name, description, price, created_at, updated_at FROM articles WHERE sku = ?`
	row := r.db.QueryRowContext(ctx, q, skuCode)
	return scanLegacyArticle(row)
}

func (r *LegacyMySQLArticleRepository) List(ctx context.Context) ([]*entities.Article, error) {
	const q = `SELECT id, sku, name, description, price, created_at, updated_at FROM articles ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entities.Article
	for rows.Next() {
		a, err := scanLegacyArticleFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *LegacyMySQLArticleRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM articles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrArticleNotFound
	}
	return nil
}

// scanLegacyArticle decodes one legacy row into an *entities.Article, crossing
// the ACL boundary: DECIMAL price -> cents, currency defaulted to EUR.
func scanLegacyArticle(row *sql.Row) (*entities.Article, error) {
	var (
		id, sku, name, desc, price string
		createdAt, updatedAt       time.Time
	)
	if err := row.Scan(&id, &sku, &name, &desc, &price, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrArticleNotFound
		}
		return nil, fmt.Errorf("scan legacy article: %w", err)
	}
	return buildLegacyArticle(id, sku, name, desc, price, createdAt, updatedAt)
}

func scanLegacyArticleFromRows(rows *sql.Rows) (*entities.Article, error) {
	var (
		id, sku, name, desc, price string
		createdAt, updatedAt       time.Time
	)
	if err := rows.Scan(&id, &sku, &name, &desc, &price, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return buildLegacyArticle(id, sku, name, desc, price, createdAt, updatedAt)
}

// buildLegacyArticle rehydrates the domain aggregate through its factories,
// the ACL's crossing point back into the clean domain.
func buildLegacyArticle(id, sku, name, desc, price string, createdAt, updatedAt time.Time) (*entities.Article, error) {
	skuVO, err := entities.NewSKU(sku)
	if err != nil {
		return nil, fmt.Errorf("rehydrate SKU: %w", err)
	}
	cents, err := decimalToCents(price)
	if err != nil {
		return nil, fmt.Errorf("rehydrate price: %w", err)
	}
	priceVO, err := entities.NewMoney(cents, legacyDefaultCurrency)
	if err != nil {
		return nil, fmt.Errorf("rehydrate Money: %w", err)
	}
	return &entities.Article{
		ID:          id,
		SKU:         *skuVO,
		Name:        name,
		Description: desc,
		Price:       *priceVO,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// centsToDecimal converts integer cents into the legacy DECIMAL string the
// legacy_db.articles.price column expects, e.g. 2999 -> "29.99". Integer/string
// math only, no float64.
func centsToDecimal(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// decimalToCents parses a legacy DECIMAL string (e.g. "29.99") back into integer
// cents. "1" -> 100, "1.5" -> 150, "29.99" -> 2999. Integer/string math only.
func decimalToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decimalToCents: invalid decimal %q: %w", s, err)
	}
	var frac int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 2 {
			return 0, fmt.Errorf("decimalToCents: too many decimal places in %q", s)
		}
		for len(fracStr) < 2 {
			fracStr += "0"
		}
		frac, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("decimalToCents: invalid decimal %q: %w", s, err)
		}
	}
	cents := whole*100 + frac
	if neg {
		cents = -cents
	}
	return cents, nil
}
