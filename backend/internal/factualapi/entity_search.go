package factualapi

import (
	"context"
	"strings"
	"unicode"
)

type EntitySearchKind string

const (
	EntitySearchSKU      EntitySearchKind = "SKU"
	EntitySearchCampaign EntitySearchKind = "CAMPAIGN"
	EntitySearchCategory EntitySearchKind = "CATEGORY"
)

type EntitySearchRequest struct {
	Query string
	Kinds []EntitySearchKind
	Limit int
}

type EntitySearchMatch struct {
	Kind        EntitySearchKind
	CanonicalID int64
	OzonSKU     *int64
	ProviderID  *int64
	Name        string
	OfferID     string
	MatchReason string
	Rank        int
	Meta        FactMeta
}

type EntitySearchResult struct {
	Items []EntitySearchMatch
}

const MaxEntitySearchLimit = 20

func NormalizeEntitySearchLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > MaxEntitySearchLimit {
		return MaxEntitySearchLimit
	}
	return limit
}

func DefaultEntitySearchKinds(kinds []EntitySearchKind) []EntitySearchKind {
	if len(kinds) == 0 {
		return []EntitySearchKind{EntitySearchSKU, EntitySearchCampaign, EntitySearchCategory}
	}
	return kinds
}

type EntitySearchTokens struct {
	Raw      string
	Numerics []int64
	Texts    []string
	Exact    string
}

var entitySearchStop = map[string]struct{}{
	"какой": {}, "какая": {}, "какие": {}, "каков": {}, "сколько": {}, "сейчас": {},
	"остаток": {}, "остатки": {}, "остатков": {}, "остатке": {},
	"у": {}, "на": {}, "по": {}, "с": {}, "из": {}, "для": {}, "и": {}, "а": {}, "the": {}, "a": {}, "an": {},
	"товар": {}, "товара": {}, "товаре": {}, "продаж": {}, "продажи": {}, "продажам": {},
	"выгрузи": {}, "файл": {}, "план": {}, "сравни": {}, "текущий": {}, "текущих": {},
	"мне": {}, "что": {}, "это": {}, "как": {}, "почему": {}, "падают": {}, "рекламу": {}, "реклама": {},
	"рекламе": {}, "если": {}, "есть": {}, "про": {}, "нему": {}, "него": {}, "ещё": {}, "еще": {},
}

func NormalizeEntitySearchQuery(query string) EntitySearchTokens {
	q := strings.ToLower(strings.TrimSpace(query))
	out := EntitySearchTokens{Raw: q, Exact: strings.Join(strings.Fields(q), " ")}
	var buf strings.Builder
	flush := func() {
		tok := strings.TrimSpace(buf.String())
		buf.Reset()
		if tok == "" {
			return
		}
		if _, stop := entitySearchStop[tok]; stop {
			return
		}
		if strings.HasPrefix(tok, "sku") {
			rest := strings.Trim(strings.TrimPrefix(tok, "sku"), "-_#")
			if rest != "" {
				tok = rest
			}
		}
		if isSearchDigits(tok) {
			n, ok := parseSearchInt(tok)
			if ok && n > 0 {
				out.Numerics = append(out.Numerics, n)
			}
			return
		}
		if len([]rune(tok)) < 2 {
			return
		}
		out.Texts = append(out.Texts, tok)
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func (t EntitySearchTokens) Empty() bool {
	return len(t.Numerics) == 0 && len(t.Texts) == 0 && strings.TrimSpace(t.Exact) == ""
}

func isSearchDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseSearchInt(s string) (int64, bool) {
	var n int64
	for _, r := range s {
		n = n*10 + int64(r-'0')
		if n > 1_000_000_000_000 {
			return 0, false
		}
	}
	return n, true
}

// SearchEntities is implemented by factual.Service; declared here for callers that
// import the request types without pulling the full API surface in tests.
type EntitySearcher interface {
	SearchEntities(context.Context, FactualReadContext, EntitySearchRequest) (EntitySearchResult, error)
}
