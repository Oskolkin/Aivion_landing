package aipipeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

const (
	GlobalContextInputSchemaVersionV1 = "global_context_generation_input_v1"
	GlobalContextInputSchemaVersionV2 = "global_context_generation_input_v2"

	// GlobalContextInputSchemaVersion is the current shared-pipeline input version.
	GlobalContextInputSchemaVersion = GlobalContextInputSchemaVersionV2
)

type GlobalContextGenerationInput struct {
	SchemaVersion string `json:"schema_version"`

	SellerAccountID int64  `json:"seller_account_id"`
	MetricDate      string `json:"metric_date"`

	DataPackSchemaVersion string `json:"data_pack_schema_version,omitempty"`
	DataPackHash          string `json:"data_pack_hash"`

	ValidatedSignals []dashboardhome.DashboardChunkSignal `json:"validated_signals,omitempty"`
	FactCatalog      map[string]aidatapack.CompactFact    `json:"fact_catalog,omitempty"`
	EntityCatalog    map[string]aidatapack.CompactEntity  `json:"entity_catalog,omitempty"`
	ManifestSummary  map[string]any                       `json:"manifest_summary,omitempty"`

	DataQualityLimitations []string `json:"data_quality_limitations,omitempty"`
	FailedChunkIDs         []string `json:"failed_chunk_ids,omitempty"`
	FailedChunkTypes       []string `json:"failed_chunk_types,omitempty"`

	ChunkSetSchemaVersion string   `json:"chunk_set_schema_version,omitempty"`
	ChunkHashes           []string `json:"chunk_hashes,omitempty"`
	SharedAnalysisStatus  string   `json:"shared_analysis_status,omitempty"`

	DataPack *aidatapack.SellerAIDataPack `json:"-"`

	AllowedCategories []string `json:"allowed_categories,omitempty"`
	ForbiddenClaims   []string `json:"forbidden_claims,omitempty"`
	MaxPrimaryRisks   int      `json:"max_primary_risks,omitempty"`
	MaxSecondaryRisks int      `json:"max_secondary_risks,omitempty"`
	MaxOpportunities  int      `json:"max_opportunities,omitempty"`
}

type globalContextGenerationInputV1Wire struct {
	SchemaVersion         string                      `json:"schema_version"`
	SellerAccountID       int64                       `json:"seller_account_id"`
	MetricDate            string                      `json:"metric_date"`
	DataPackSchemaVersion string                      `json:"data_pack_schema_version,omitempty"`
	DataPackHash          string                      `json:"data_pack_hash"`
	DataPack              aidatapack.SellerAIDataPack `json:"data_pack"`
	AllowedCategories     []string                    `json:"allowed_categories"`
	ForbiddenClaims       []string                    `json:"forbidden_claims,omitempty"`
	MaxPrimaryRisks       int                         `json:"max_primary_risks,omitempty"`
	MaxSecondaryRisks     int                         `json:"max_secondary_risks,omitempty"`
	MaxOpportunities      int                         `json:"max_opportunities,omitempty"`
}

type globalContextGenerationInputV2Wire struct {
	SchemaVersion          string                               `json:"schema_version"`
	SellerAccountID        int64                                `json:"seller_account_id"`
	MetricDate             string                               `json:"metric_date"`
	DataPackHash           string                               `json:"data_pack_hash"`
	ValidatedSignals       []dashboardhome.DashboardChunkSignal `json:"validated_signals"`
	FactCatalog            map[string]aidatapack.CompactFact    `json:"fact_catalog"`
	EntityCatalog          map[string]aidatapack.CompactEntity  `json:"entity_catalog"`
	ManifestSummary        map[string]any                       `json:"manifest_summary"`
	DataQualityLimitations []string                             `json:"data_quality_limitations"`
	FailedChunkIDs         []string                             `json:"failed_chunk_ids"`
	FailedChunkTypes       []string                             `json:"failed_chunk_types"`
	ChunkSetSchemaVersion  string                               `json:"chunk_set_schema_version,omitempty"`
	ChunkHashes            []string                             `json:"chunk_hashes,omitempty"`
	SharedAnalysisStatus   string                               `json:"shared_analysis_status,omitempty"`
	AllowedCategories      []string                             `json:"allowed_categories,omitempty"`
	ForbiddenClaims        []string                             `json:"forbidden_claims,omitempty"`
	MaxPrimaryRisks        int                                  `json:"max_primary_risks,omitempty"`
	MaxSecondaryRisks      int                                  `json:"max_secondary_risks,omitempty"`
	MaxOpportunities       int                                  `json:"max_opportunities,omitempty"`
}

// DefaultGlobalContextGenerationInput builds the legacy V1 input with full Data Pack.
func DefaultGlobalContextGenerationInput(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
) GlobalContextGenerationInput {
	return GlobalContextGenerationInput{
		SchemaVersion:         GlobalContextInputSchemaVersionV1,
		SellerAccountID:       pack.SellerContext.SellerAccountID,
		MetricDate:            pack.SellerContext.MetricDate,
		DataPackSchemaVersion: pack.SchemaVersion,
		DataPackHash:          dataPackHash,
		DataPack:              pack,
		AllowedCategories:     append([]string(nil), aicontext.AllowedCategories...),
		ForbiddenClaims:       append([]string(nil), defaultForbiddenClaims...),
		MaxPrimaryRisks:       4,
		MaxSecondaryRisks:     5,
		MaxOpportunities:      3,
	}
}

// BuildGlobalContextGenerationInput builds the shared-pipeline V2 input from chunk analysis.
func BuildGlobalContextGenerationInput(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
) (GlobalContextGenerationInput, error) {
	if pack == nil {
		return GlobalContextGenerationInput{}, fmt.Errorf("data pack is nil")
	}
	if strings.TrimSpace(dataPackHash) == "" {
		return GlobalContextGenerationInput{}, fmt.Errorf("data_pack_hash is required")
	}
	if chunkSet == nil {
		return GlobalContextGenerationInput{}, fmt.Errorf("chunk set is nil")
	}
	if chunkRun == nil {
		return GlobalContextGenerationInput{}, fmt.Errorf("chunk analysis result is nil")
	}
	if chunkRun.SellerAccountID != pack.SellerContext.SellerAccountID {
		return GlobalContextGenerationInput{}, fmt.Errorf("seller_account_id mismatch")
	}
	if chunkRun.MetricDate != pack.SellerContext.MetricDate {
		return GlobalContextGenerationInput{}, fmt.Errorf("metric_date mismatch")
	}
	if chunkRun.ChunkCount != len(chunkSet.Chunks) {
		return GlobalContextGenerationInput{}, fmt.Errorf("chunk_count mismatch")
	}
	if chunkRun.Completed <= 0 {
		return GlobalContextGenerationInput{}, fmt.Errorf("shared chunk analysis has no completed chunks")
	}
	switch chunkRun.AnalysisStatus {
	case dashboardhome.ChunkAnalysisStatusCompleted, dashboardhome.ChunkAnalysisStatusPartial:
	default:
		return GlobalContextGenerationInput{}, fmt.Errorf("shared analysis status %q is not reusable", chunkRun.AnalysisStatus)
	}
	if err := validateSharedChunkAnalysisRun(chunkSet, chunkRun); err != nil {
		return GlobalContextGenerationInput{}, err
	}

	signals := normalizeSignals(chunkRun.ValidatedSignals)
	factIDs := collectSignalFactIDs(signals)
	factCatalog, err := aidatapack.BuildCompactFactCatalog(pack, factIDs)
	if err != nil {
		return GlobalContextGenerationInput{}, err
	}
	entityIDs := collectSignalEntityIDs(signals)
	entityIDs = appendUniqueSorted(entityIDs, entityIDsFromFacts(pack, factIDs)...)
	entityCatalog, err := aidatapack.BuildCompactEntityCatalog(pack, entityIDs)
	if err != nil {
		return GlobalContextGenerationInput{}, err
	}

	chunkHashes, err := orderedChunkHashes(chunkSet)
	if err != nil {
		return GlobalContextGenerationInput{}, err
	}

	input := GlobalContextGenerationInput{
		SchemaVersion:          GlobalContextInputSchemaVersionV2,
		SellerAccountID:        pack.SellerContext.SellerAccountID,
		MetricDate:             pack.SellerContext.MetricDate,
		DataPackSchemaVersion:  pack.SchemaVersion,
		DataPackHash:           dataPackHash,
		ValidatedSignals:       signals,
		FactCatalog:            factCatalog,
		EntityCatalog:          entityCatalog,
		ManifestSummary:        buildGlobalContextManifestSummary(pack, chunkSet, chunkRun, len(factCatalog), len(entityCatalog)),
		DataQualityLimitations: mergeGlobalContextLimitations(pack, chunkRun),
		FailedChunkIDs:         normalizeStringSlice(chunkRun.FailedChunkIDs),
		FailedChunkTypes:       normalizeStringSlice(chunkRun.FailedChunkTypes),
		ChunkSetSchemaVersion:  chunkSet.SchemaVersion,
		ChunkHashes:            chunkHashes,
		SharedAnalysisStatus:   chunkRun.AnalysisStatus,
		DataPack:               pack,
		AllowedCategories:      append([]string(nil), aicontext.AllowedCategories...),
		ForbiddenClaims:        append([]string(nil), defaultForbiddenClaims...),
		MaxPrimaryRisks:        4,
		MaxSecondaryRisks:      5,
		MaxOpportunities:       3,
	}
	normalizeGlobalContextInputCollections(&input)
	return input, nil
}

func normalizeGlobalContextInputCollections(input *GlobalContextGenerationInput) {
	if input == nil {
		return
	}
	if input.ValidatedSignals == nil {
		input.ValidatedSignals = []dashboardhome.DashboardChunkSignal{}
	}
	if input.FactCatalog == nil {
		input.FactCatalog = map[string]aidatapack.CompactFact{}
	}
	if input.EntityCatalog == nil {
		input.EntityCatalog = map[string]aidatapack.CompactEntity{}
	}
	if input.ManifestSummary == nil {
		input.ManifestSummary = map[string]any{}
	}
	if input.DataQualityLimitations == nil {
		input.DataQualityLimitations = []string{}
	}
	if input.FailedChunkIDs == nil {
		input.FailedChunkIDs = []string{}
	}
	if input.FailedChunkTypes == nil {
		input.FailedChunkTypes = []string{}
	}
	if input.ChunkHashes == nil {
		input.ChunkHashes = []string{}
	}
}

func collectSignalFactIDs(signals []dashboardhome.DashboardChunkSignal) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, signal := range signals {
		for _, raw := range signal.FactIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func collectSignalEntityIDs(signals []dashboardhome.DashboardChunkSignal) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, signal := range signals {
		for _, raw := range signal.EntityIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func entityIDsFromFacts(pack *aidatapack.SellerAIDataPack, factIDs []string) []string {
	if pack == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, factID := range factIDs {
		fact, ok := pack.FactRegistry[factID]
		if !ok {
			continue
		}
		eid := strings.TrimSpace(fact.EntityID)
		if eid == "" {
			continue
		}
		if _, ok := seen[eid]; ok {
			continue
		}
		seen[eid] = struct{}{}
		out = append(out, eid)
	}
	sort.Strings(out)
	return out
}

func appendUniqueSorted(base []string, extra ...string) []string {
	seen := map[string]struct{}{}
	for _, id := range base {
		seen[id] = struct{}{}
	}
	for _, id := range extra {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		base = append(base, id)
	}
	sort.Strings(base)
	return base
}

func mergeGlobalContextLimitations(
	pack *aidatapack.SellerAIDataPack,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if pack != nil {
		for _, lim := range pack.DataQuality.Limitations {
			add(lim)
		}
	}
	if chunkRun != nil && chunkRun.AnalysisStatus == dashboardhome.ChunkAnalysisStatusPartial {
		add(sharedChunkAnalysisPartialLimitation)
	}
	if chunkRun != nil && chunkRun.Failed > 0 {
		add("Часть чанков не была проанализирована; выводы по ним недоступны.")
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func buildGlobalContextManifestSummary(
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
	factCount, entityCount int,
) map[string]any {
	summary := map[string]any{
		"fact_catalog_count":   factCount,
		"entity_catalog_count": entityCount,
	}
	if pack != nil {
		summary["data_pack_schema_version"] = pack.SchemaVersion
		summary["metric_date"] = pack.SellerContext.MetricDate
		if pack.DataWindows.Last7d.From != "" {
			summary["period_start"] = pack.DataWindows.Last7d.From
			summary["period_end"] = pack.DataWindows.Last7d.To
		}
		summary["manifest_limitations_count"] = len(pack.DataQuality.Limitations)
	}
	if chunkSet != nil {
		summary["chunk_set_schema_version"] = chunkSet.SchemaVersion
		summary["chunk_count"] = len(chunkSet.Chunks)
		sourceTables := make([]string, 0, len(chunkSet.Chunks))
		for _, ch := range chunkSet.Chunks {
			if ch.SourceTable != "" {
				sourceTables = append(sourceTables, ch.SourceTable)
			}
		}
		sort.Strings(sourceTables)
		summary["source_tables"] = sourceTables
	}
	if chunkRun != nil {
		summary["completed_chunk_count"] = chunkRun.Completed
		summary["failed_chunk_count"] = chunkRun.Failed
		summary["validated_signal_count"] = chunkRun.ValidatedSignalCount
		summary["shared_analysis_status"] = chunkRun.AnalysisStatus
	}
	return summary
}

// MarshalGlobalContextGenerationInputWire returns the JSON payload sent to the AI provider.
func MarshalGlobalContextGenerationInputWire(input GlobalContextGenerationInput) ([]byte, error) {
	switch input.SchemaVersion {
	case GlobalContextInputSchemaVersionV1:
		if input.DataPack == nil {
			return nil, fmt.Errorf("legacy global context input requires data pack")
		}
		return json.Marshal(globalContextGenerationInputV1Wire{
			SchemaVersion:         input.SchemaVersion,
			SellerAccountID:       input.SellerAccountID,
			MetricDate:            input.MetricDate,
			DataPackSchemaVersion: input.DataPackSchemaVersion,
			DataPackHash:          input.DataPackHash,
			DataPack:              *input.DataPack,
			AllowedCategories:     input.AllowedCategories,
			ForbiddenClaims:       input.ForbiddenClaims,
			MaxPrimaryRisks:       input.MaxPrimaryRisks,
			MaxSecondaryRisks:     input.MaxSecondaryRisks,
			MaxOpportunities:      input.MaxOpportunities,
		})
	case GlobalContextInputSchemaVersionV2:
		normalizeGlobalContextInputCollections(&input)
		return json.Marshal(globalContextGenerationInputV2Wire{
			SchemaVersion:          input.SchemaVersion,
			SellerAccountID:        input.SellerAccountID,
			MetricDate:             input.MetricDate,
			DataPackHash:           input.DataPackHash,
			ValidatedSignals:       input.ValidatedSignals,
			FactCatalog:            input.FactCatalog,
			EntityCatalog:          input.EntityCatalog,
			ManifestSummary:        input.ManifestSummary,
			DataQualityLimitations: input.DataQualityLimitations,
			FailedChunkIDs:         input.FailedChunkIDs,
			FailedChunkTypes:       input.FailedChunkTypes,
			ChunkSetSchemaVersion:  input.ChunkSetSchemaVersion,
			ChunkHashes:            input.ChunkHashes,
			SharedAnalysisStatus:   input.SharedAnalysisStatus,
			AllowedCategories:      input.AllowedCategories,
			ForbiddenClaims:        input.ForbiddenClaims,
			MaxPrimaryRisks:        input.MaxPrimaryRisks,
			MaxSecondaryRisks:      input.MaxSecondaryRisks,
			MaxOpportunities:       input.MaxOpportunities,
		})
	default:
		return nil, fmt.Errorf("unsupported global context input schema %q", input.SchemaVersion)
	}
}

// HashGlobalContextGenerationInput hashes the wire payload for artifact compatibility checks.
func HashGlobalContextGenerationInput(input GlobalContextGenerationInput) (string, error) {
	raw, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		return "", err
	}
	var wire any
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", err
	}
	return aihash.HashJSON(wire)
}

// MarshalGlobalContextInput returns JSON bytes for debug artifacts.
func MarshalGlobalContextInput(input GlobalContextGenerationInput) ([]byte, error) {
	return MarshalGlobalContextGenerationInputWire(input)
}

var defaultForbiddenClaims = []string{
	"не считать маржинальность / прибыль",
	"не делать repricing",
	"не делать SKU-level рекламные выводы",
	"не обещать точный финансовый эффект",
	"не предлагать автопаузу рекламы",
	"не использовать факты вне Data Pack",
	"не трактовать missing data как zero",
}
