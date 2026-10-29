package actions

import "strings"

// AIFirstActionScopeType declares whether an Action applies to the whole account,
// specific entities, or both (mixed). AI must set this explicitly; backend never
// infers scope from action type.
type AIFirstActionScopeType string

const (
	AIFirstActionScopeAccount AIFirstActionScopeType = "account"
	AIFirstActionScopeEntity  AIFirstActionScopeType = "entity"
	AIFirstActionScopeMixed   AIFirstActionScopeType = "mixed"
)

// AIFirstObjectActionsCoverage is optional AI metadata about object_actions coverage.
type AIFirstObjectActionsCoverage struct {
	Mode   string `json:"mode"` // full | partial | none
	Reason string `json:"reason,omitempty"`
}

// ActionObjectActionsCoverage is the persisted/API object_actions coverage metadata.
type ActionObjectActionsCoverage struct {
	Mode   string `json:"mode"`
	Reason string `json:"reason,omitempty"`
}

func normalizeAIFirstScopeType(raw string) AIFirstActionScopeType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(AIFirstActionScopeAccount):
		return AIFirstActionScopeAccount
	case string(AIFirstActionScopeEntity):
		return AIFirstActionScopeEntity
	case string(AIFirstActionScopeMixed):
		return AIFirstActionScopeMixed
	default:
		return ""
	}
}

func isValidAIFirstScopeType(scope AIFirstActionScopeType) bool {
	switch scope {
	case AIFirstActionScopeAccount, AIFirstActionScopeEntity, AIFirstActionScopeMixed:
		return true
	default:
		return false
	}
}

func draftActionHasAuthoritativeEntities(action AIFirstDraftAction) bool {
	if len(action.AffectedEntities) > 0 {
		for _, ent := range action.AffectedEntities {
			if strings.TrimSpace(ent.EntityID) != "" {
				return true
			}
		}
	}
	if action.OperationalDetail != nil && len(action.OperationalDetail.ObjectActions) > 0 {
		return true
	}
	return false
}

// resolvePersistScopeType returns scope for persist guard. Legacy actions without
// scope_type fall back to entity when entities exist, otherwise account.
func resolvePersistScopeType(item ActionItem) AIFirstActionScopeType {
	if scope := normalizeAIFirstScopeType(item.ScopeType); scope != "" {
		return scope
	}
	if hasAffectedEntitiesPreview(item.AffectedEntities) {
		return AIFirstActionScopeEntity
	}
	return AIFirstActionScopeAccount
}

func mapObjectActionsCoverageToAPI(coverage *AIFirstObjectActionsCoverage) *ActionObjectActionsCoverage {
	if coverage == nil || strings.TrimSpace(coverage.Mode) == "" {
		return nil
	}
	return &ActionObjectActionsCoverage{
		Mode:   strings.ToLower(strings.TrimSpace(coverage.Mode)),
		Reason: strings.TrimSpace(coverage.Reason),
	}
}

func inferObjectActionsCoverageMode(total, displayed int) string {
	if total <= 0 {
		return "none"
	}
	if displayed <= 0 {
		return "none"
	}
	if displayed >= total {
		return "full"
	}
	return "partial"
}

func objectActionsCoverageMetadataMismatch(coverage *AIFirstObjectActionsCoverage, total, displayed int) bool {
	if coverage == nil || strings.TrimSpace(coverage.Mode) == "" {
		return false
	}
	actual := inferObjectActionsCoverageMode(total, displayed)
	return strings.ToLower(strings.TrimSpace(coverage.Mode)) != actual
}
