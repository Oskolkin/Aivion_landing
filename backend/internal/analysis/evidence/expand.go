package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type ExpandInput struct {
	SellerID          int64
	UniverseHash      string
	AnalysisDate      time.Time
	CutoffAt          time.Time
	Validated         discovery.ValidatedOutputV1
	Continuity        continuity.ContinuityContextV1
	ValidatedHash     string
	DiscoveryPackHash string
	FactHandleMap     discovery.FactHandleMapV1
	FactHandleMapHash string
	EvidenceMenuHash  string
	Sealed            factualapi.FactualReadContext
	Registry          *Registry
	Budget            BudgetProfile
}

type ExpandOutput struct {
	RequestSet   RequestSetV1
	Bundle       BundleV1
	Handles      UnifiedHandleRegistryV1
	Report       ExpansionReportV1
	BundleBytes  []byte
	RequestBytes []byte
	HandleBytes  []byte
	ReportBytes  []byte
	Limited      bool
}

func Expand(ctx context.Context, in ExpandInput) (ExpandOutput, error) {
	if in.Sealed.ContextHash() != "" && in.UniverseHash != "" && in.Sealed.SellerAccountID() != in.SellerID {
		return ExpandOutput{}, Integrity("seller mismatch")
	}
	if in.Registry == nil {
		return ExpandOutput{}, Integrity("resolver registry missing")
	}
	rawN := len(CollectRawRequests(in.Validated)) + CountRequiredContinuitySupports(in.Continuity)
	norm, rawCounted, err := NormalizeAllRequests(in.UniverseHash, in.Validated, in.Continuity)
	if rawCounted != rawN {
		return ExpandOutput{}, Integrity("raw request count mismatch")
	}
	if err != nil {
		return ExpandOutput{}, err
	}
	if len(norm) > in.Budget.MaxUniqueRequests {
		return ExpandOutput{}, BudgetExceeded("unique requests exceed contract max")
	}
	grouped := map[string][]NormalizedEvidenceRequest{}
	for _, n := range norm {
		grouped[n.EvidenceType] = append(grouped[n.EvidenceType], n)
	}
	resultsByKey := map[string]MaterializationResult{}
	timings := map[string]int{}
	apiCounts := map[string]int{}
	startAll := time.Now()
	for _, t := range evidencecatalog.Enabled() {
		reqs := grouped[t.Key]
		if len(reqs) == 0 {
			continue
		}
		res, ok := in.Registry.Get(t.Key)
		if !ok {
			return ExpandOutput{}, Integrity("resolver missing for enabled type " + t.Key)
		}
		st := time.Now()
		out := res.ResolveBatch(ctx, in.Sealed, reqs)
		timings[t.Key] = int(time.Since(st).Milliseconds())
		if len(out) != len(reqs) {
			return ExpandOutput{}, Integrity("resolver result count mismatch for " + t.Key)
		}
		for i, r := range out {
			if r.Integrity != nil {
				return ExpandOutput{}, r.Integrity
			}
			if r.Request.RequestKey == "" {
				r.Request = reqs[i]
			}
			resultsByKey[r.Request.RequestKey] = r
			apiCounts[t.Key] += r.APICalls
			if r.APICalls == 0 {
				apiCounts[t.Key]++
			}
		}
	}
	_ = startAll
	items := make([]EvidenceItem, 0, len(norm))
	sum := BundleSummary{Requested: rawN, Unique: len(norm)}
	limited := false
	truncN := 0
	byType := map[string]int{}
	matResults := make([]MaterializationResult, 0, len(norm))
	for _, n := range norm {
		r, ok := resultsByKey[n.RequestKey]
		if !ok {
			return ExpandOutput{}, Integrity("dropped request " + n.RequestKey)
		}
		data, err := marshalData(r.Data)
		if err != nil {
			return ExpandOutput{}, err
		}
		item := EvidenceItem{
			EvidenceID: EvidenceID(n.Ordinal), EvidenceType: n.EvidenceType, EvidenceTypeVersion: n.EvidenceVersion,
			Subject: n.Subject, Related: n.Related, Readiness: r.Readiness, MaterializationStatus: r.Status,
			TemporalScope: r.Temporal, Coverage: r.Coverage, Limitations: r.Limitations, Truncation: r.Truncation,
			Data: data,
		}
		if item.Limitations == nil {
			item.Limitations = []Limitation{}
		}
		if err = ValidateItem(item, n, in.SellerID, in.UniverseHash); err != nil {
			return ExpandOutput{}, err
		}
		switch r.Status {
		case StatusComplete:
			sum.Complete++
		case StatusEmptyValid:
			sum.EmptyValid++
		case StatusPartial:
			sum.Partial++
			limited = true
		case StatusFailed:
			sum.Failed++
			limited = true
		}
		if r.Readiness == "UNAVAILABLE" || r.Readiness == "STALE" || r.Readiness == "PARTIAL" || r.Readiness == "FAILED" {
			limited = true
		}
		if r.Truncation.Truncated {
			truncN++
		}
		byType[n.EvidenceType]++
		items = append(items, item)
		matResults = append(matResults, r)
	}
	handles, items, err := assignHandles(items, matResults, in.FactHandleMap)
	if err != nil {
		return ExpandOutput{}, err
	}
	rels := []ThreadRequestRelation{}
	cons := []EvidenceConsumerRelation{}
	for _, n := range norm {
		id := EvidenceID(n.Ordinal)
		for _, tr := range n.ThreadRefs {
			rels = append(rels, ThreadRequestRelation{ThreadID: tr.ThreadID, RequestKey: n.RequestKey, EvidenceID: id})
		}
		for _, cr := range n.ConsumerRefs {
			cons = append(cons, EvidenceConsumerRelation{
				Source: cr.Source, RequestKey: n.RequestKey, EvidenceID: id,
				ThreadRef: cr.ThreadRef, PreviousCaseRef: cr.PreviousCaseRef,
				PreviousProposalRef: cr.PreviousProposalRef, SupportKey: cr.SupportKey,
			})
		}
	}
	contRels, err := BuildContinuityRelations(in.Continuity, norm, items)
	if err != nil {
		return ExpandOutput{}, err
	}
	if err = ValidateContinuityCoverage(in.Continuity, contRels); err != nil {
		return ExpandOutput{}, err
	}
	schemaSet := RequestSetSchemaV2
	bundleSchema := BundleSchemaV2
	normVer := RequestNormalizationV2
	reqSet := RequestSetV1{
		SchemaVersion: schemaSet, FactualUniverseHash: in.UniverseHash, ValidatedDiscoveryHash: in.ValidatedHash,
		CatalogVersion: CatalogV1, ResolverRegistryVersion: ResolverRegistryV1, NormalizationVersion: normVer,
		RawRequestCount: rawN, UniqueRequestCount: len(norm), Requests: norm, ThreadRelations: rels,
		ConsumerRelations: cons, ContinuityRelations: contRels,
	}
	bundle := BundleV1{
		SchemaVersion: bundleSchema, FactualUniverseHash: in.UniverseHash,
		AnalysisDate: in.AnalysisDate.Format("2006-01-02"), AnalysisCutoffAt: in.CutoffAt.UTC().Format(time.RFC3339),
		CatalogVersion: CatalogV1, ResolverRegistryVersion: ResolverRegistryV1, BudgetProfileVersion: in.Budget.Version,
		FactHandleRegistryVersion: FactHandleRegistryV1, Items: items, ThreadEvidenceRelations: rels,
		ContinuitySupportRelations: contRels, Summary: sum,
	}
	bundleBytes, err := MarshalBundleCanonical(bundle)
	if err != nil {
		return ExpandOutput{}, err
	}
	if len(bundleBytes) > in.Budget.MaxBundleBytes {
		return ExpandOutput{}, BudgetExceeded(fmt.Sprintf("bundle %d bytes exceeds %d", len(bundleBytes), in.Budget.MaxBundleBytes))
	}
	reqBytes, _ := canonicaljson.Marshal(reqSet)
	hBytes, _ := canonicaljson.Marshal(handles)
	tokens := len(bundleBytes) / 4
	ratio := "1.00"
	if rawN > 0 {
		ratio = strconv.FormatFloat(float64(len(norm))/float64(rawN), 'f', 2, 64)
	}
	rep := ExpansionReportV1{
		SchemaVersion: ExpansionReportSchemaV1, RawRequests: rawN, UniqueRequests: len(norm), DedupRatio: ratio,
		ItemsComplete: sum.Complete, ItemsEmptyValid: sum.EmptyValid, ItemsPartial: sum.Partial, ItemsFailed: sum.Failed,
		ItemsByType: byType, Bytes: len(bundleBytes), EstimatedTokens: tokens, TruncationCounts: truncN,
		ResolverTimingsMS: timings, FactualAPICallCounts: apiCounts,
	}
	repBytes, _ := canonicaljson.Marshal(rep)
	ObserveExpansion(rep, limited)
	return ExpandOutput{
		RequestSet: reqSet, Bundle: bundle, Handles: handles, Report: rep,
		BundleBytes: bundleBytes, RequestBytes: reqBytes, HandleBytes: hBytes, ReportBytes: repBytes, Limited: limited,
	}, nil
}

func MustJSON(v any) json.RawMessage {
	b, _ := canonicaljson.Marshal(v)
	return b
}
