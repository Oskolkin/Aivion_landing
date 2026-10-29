package descriptioncategories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const internalMetaTreeSyncedAt = "ozon_description_category_tree_synced_at_rfc3339"

type ozonTreeClient interface {
	GetDescriptionCategoryTree(ctx context.Context, clientID, apiKey string) (*ozon.TypedResponse[[]ozon.DescriptionCategoryTreeJSONNode], error)
}

// FlatRow is a flattened Ozon category/type node for persistence.
type FlatRow struct {
	ID       int64
	ParentID int64
	Name     string
	Path     string
}

// SyncGlobalIfStale downloads the global Ozon description category tree using seller credentials
// and upserts rows into description_categories. It is throttled via internal_meta (default 24h).
func SyncGlobalIfStale(ctx context.Context, db *pgxpool.Pool, api ozonTreeClient, clientID, apiKey string, minInterval time.Duration) (int, error) {
	if minInterval <= 0 {
		minInterval = 24 * time.Hour
	}
	q := dbgen.New(db)
	val, err := q.GetInternalMetaValue(ctx, internalMetaTreeSyncedAt)
	if err == nil && strings.TrimSpace(val) != "" {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(val)); parseErr == nil {
			if time.Since(t) < minInterval {
				return 0, nil
			}
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("read internal meta: %w", err)
	}

	resp, err := api.GetDescriptionCategoryTree(ctx, clientID, apiKey)
	if err != nil {
		return 0, err
	}
	rows := FlattenTree(resp.Data)
	for _, r := range rows {
		var pid pgtype.Int8
		if r.ParentID > 0 {
			pid = pgtype.Int8{Int64: r.ParentID, Valid: true}
		}
		if err := q.UpsertDescriptionCategory(ctx, dbgen.UpsertDescriptionCategoryParams{
			DescriptionCategoryID: r.ID,
			ParentID:              pid,
			CategoryName:          r.Name,
			Path:                  nullText(r.Path),
		}); err != nil {
			return 0, fmt.Errorf("upsert description category %d: %w", r.ID, err)
		}
	}
	metaVal := time.Now().UTC().Format(time.RFC3339)
	if err := q.UpsertInternalMetaValue(ctx, dbgen.UpsertInternalMetaValueParams{
		Key:   internalMetaTreeSyncedAt,
		Value: metaVal,
	}); err != nil {
		return 0, fmt.Errorf("save internal meta: %w", err)
	}
	return len(rows), nil
}

// FlattenTree walks the Ozon tree and emits stable rows (category and type ids).
func FlattenTree(roots []ozon.DescriptionCategoryTreeJSONNode) []FlatRow {
	var out []FlatRow
	for i := range roots {
		walkTreeNode(roots[i], 0, nil, &out)
	}
	return out
}

func walkTreeNode(n ozon.DescriptionCategoryTreeJSONNode, parentID int64, path []string, out *[]FlatRow) {
	id, name, ok := resolveTreeNode(n)
	nextParent := parentID
	nextPath := path
	if ok && id != 0 && strings.TrimSpace(name) != "" {
		segs := append(append([]string{}, path...), name)
		*out = append(*out, FlatRow{
			ID:       id,
			ParentID: parentID,
			Name:     name,
			Path:     strings.Join(segs, " / "),
		})
		nextParent = id
		nextPath = segs
	}
	for _, raw := range n.Children {
		var ch ozon.DescriptionCategoryTreeJSONNode
		if err := json.Unmarshal(raw, &ch); err != nil {
			continue
		}
		walkTreeNode(ch, nextParent, nextPath, out)
	}
}

func resolveTreeNode(n ozon.DescriptionCategoryTreeJSONNode) (id int64, name string, ok bool) {
	if n.TypeID != 0 {
		nm := strings.TrimSpace(n.TypeName)
		if nm == "" {
			nm = strings.TrimSpace(n.CategoryName)
		}
		if nm != "" {
			return n.TypeID, nm, true
		}
	}
	if n.DescriptionCategoryID != 0 {
		return n.DescriptionCategoryID, strings.TrimSpace(n.CategoryName), true
	}
	return 0, "", false
}

func nullText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
