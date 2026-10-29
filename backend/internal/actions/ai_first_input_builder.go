package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
)

// AIDataPackBuilder builds the Seller AI Data Pack from DB state.
// Implemented by aidatapack.Service.
type AIDataPackBuilder interface {
	Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error)
}

// aiFirstForbiddenClaims are guardrails passed to the AI-first generator.
var aiFirstForbiddenClaims = []string{
	"Не давать ценовые рекомендации и не обещать repricing.",
	"Не считать маржинальность, себестоимость или точную прибыль.",
	"Не делать SKU-level рекламные выводы: реклама оценивается только на уровне кампаний.",
	"Не обещать точный финансовый эффект.",
	"Не добавлять SKU, кампании, категории или числа, которых нет в Data Pack.",
	"Не предлагать автопаузу или автоматическое изменение ставок без подтверждения пользователя.",
	"Не использовать отсутствующие строки метрик как нулевые продажи.",
}

// BuildActionsAIFirstInput assembles the AI-first generation input from the
// Seller AI Data Pack. It never uses CandidateBuilder or backend-generated
// candidates: the backend only prepares facts, allowed taxonomy and limits.
func (s *GenerationService) BuildActionsAIFirstInput(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts GenerateActionsOptions,
) (ActionsAIFirstInput, error) {
	if s.dataPackBuilder == nil {
		return ActionsAIFirstInput{}, fmt.Errorf("ai-first: data pack builder is not configured")
	}
	pack, err := s.dataPackBuilder.Build(ctx, sellerAccountID, metricDate)
	if err != nil {
		return ActionsAIFirstInput{}, fmt.Errorf("ai-first: build data pack: %w", err)
	}
	if err := aidatapack.ValidateDataPack(pack); err != nil {
		return ActionsAIFirstInput{}, fmt.Errorf("ai-first: invalid data pack: %w", err)
	}
	return BuildActionsAIFirstInputFromDataPack(pack, aihash.HashDataPack(pack), nil, opts)
}

// BuildActionsAIFirstInputFromDataPack assembles AI-first input from a prebuilt
// Data Pack without DB access or candidate building.
func BuildActionsAIFirstInputFromDataPack(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	opts GenerateActionsOptions,
) (ActionsAIFirstInput, error) {
	if pack == nil {
		return ActionsAIFirstInput{}, fmt.Errorf("ai-first: data pack is nil")
	}
	if dataPackHash == "" {
		dataPackHash = aihash.HashDataPack(pack)
	}

	maxActions := opts.MaxActionsOutput
	if maxActions <= 0 {
		maxActions = DefaultGenerateActionsOptions().MaxActionsOutput
	}

	return ActionsAIFirstInput{
		SchemaVersion:         ActionsAIFirstSchemaVersionV1,
		SellerAccountID:       pack.SellerContext.SellerAccountID,
		MetricDate:            pack.SellerContext.MetricDate,
		DataPackSchemaVersion: pack.SchemaVersion,
		DataPackHash:          dataPackHash,
		DataPack:              pack,
		GlobalContext:         globalContext,
		AllowedActionTypes:    append([]ActionType(nil), ActionsAIFirstActiveActionTypes...),
		AllowedCategories:     append([]ActionCategory(nil), ActionsAIFirstActiveCategories...),
		AllowedPriorities:     []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch},
		AllowedCTAKeys:        nil,
		ForbiddenClaims:       append([]string(nil), aiFirstForbiddenClaims...),
		MaxActions:            maxActions,
	}, nil
}

func hashDataPack(pack *aidatapack.SellerAIDataPack) string {
	return aihash.HashDataPack(pack)
}
