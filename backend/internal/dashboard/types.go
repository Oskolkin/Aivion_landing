package dashboard

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"

type HomeResponse struct {
	State       string                    `json:"state"`
	Publication *PublicationMeta          `json:"publication"`
	PreAnalysis *PreAnalysis              `json:"pre_analysis,omitempty"`
	Periods     map[string]any            `json:"periods,omitempty"`
	Limitations []string                  `json:"limitations,omitempty"`
	Overview    *projection.OverviewV1DTO `json:"overview,omitempty"`
	Funnel      *projection.FunnelV1DTO   `json:"funnel,omitempty"`
	Stock       *projection.DomainPageDTO `json:"stock,omitempty"`
	Advertising *projection.DomainPageDTO `json:"advertising,omitempty"`
	Focus       []FocusGroup              `json:"focus,omitempty"`
	Error       *ProductError             `json:"error,omitempty"`
}

type PublicationMeta struct {
	ID               string `json:"id"`
	AnalysisDate     string `json:"analysis_date"`
	AnalysisCutoffAt string `json:"analysis_cutoff_at"`
	PublishedAt      string `json:"published_at"`
}

type PreAnalysis struct {
	ReasonCode string `json:"reason_code"`
	MessageKey string `json:"message_key"`
}

type ProductError struct {
	ReasonCode string `json:"reason_code"`
	MessageKey string `json:"message_key"`
}

type FocusGroup struct {
	ActionViewID   string `json:"action_view_id"`
	DisplayOrder   int    `json:"display_order"`
	DisplayTitle   string `json:"display_title"`
	DisplaySummary string `json:"display_summary"`
	ActionType     string `json:"action_type"`
	Family         string `json:"family"`
	ActiveMembers  int    `json:"active_member_count"`
}

type RowsResponse struct {
	PublicationID string           `json:"publication_id"`
	Items         []map[string]any `json:"items"`
	NextCursor    string           `json:"next_cursor,omitempty"`
	HasMore       bool             `json:"has_more"`
}
