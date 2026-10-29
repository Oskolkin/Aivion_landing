package actionview

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const cursorVersion = 1
const sortAivion = "aivion_display_order"

var ErrCursorMismatch = errors.New("cursor_mismatch")

type pageCursor struct {
	V      int    `json:"v"`
	Seller int64  `json:"s"`
	Status string `json:"st"`
	Family string `json:"f"`
	Query  string `json:"q"`
	Sort   string `json:"so"`
	Order  int    `json:"o"`
	ID     string `json:"id"`
}

func normalizeStatus(status string) string {
	if status == "" {
		return "active"
	}
	if status == "dismissed" {
		return "irrelevant"
	}
	return status
}

func encodeCursor(c pageCursor) string {
	c.V = cursorVersion
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (pageCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return pageCursor{}, fmt.Errorf("%w: decode", ErrCursorMismatch)
	}
	var c pageCursor
	if err := json.Unmarshal(b, &c); err != nil || c.V != cursorVersion || c.ID == "" {
		return pageCursor{}, fmt.Errorf("%w: payload", ErrCursorMismatch)
	}
	return c, nil
}

func (c pageCursor) matches(seller int64, f ListFilter) bool {
	return c.Seller == seller &&
		c.Status == normalizeStatus(f.Status) &&
		c.Family == f.Family &&
		c.Query == f.Query &&
		c.Sort == sortAivion
}

type orderedView struct {
	order int
	id    string
}

func afterCursor(items []orderedView, cur pageCursor) int {
	for i, it := range items {
		if it.order > cur.Order || (it.order == cur.Order && it.id > cur.ID) {
			return i
		}
	}
	return len(items)
}

func sortKeys(order int, id string) orderedView {
	return orderedView{order: order, id: id}
}

func BindCursor(sellerID int64, f ListFilter, order int, id string) string {
	return encodeCursor(pageCursor{
		Seller: sellerID, Status: normalizeStatus(f.Status), Family: f.Family, Query: f.Query,
		Sort: sortAivion, Order: order, ID: id,
	})
}

func sortGroupViews(in []GroupView) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].DisplayOrder != in[j].DisplayOrder {
			return in[i].DisplayOrder < in[j].DisplayOrder
		}
		return in[i].ActionViewID < in[j].ActionViewID
	})
}

func sortHistoryViews(in []CaseHistoryView) {
	sort.SliceStable(in, func(i, j int) bool {
		return in[i].ActionViewID < in[j].ActionViewID
	})
}
