package actionview

type ListResponse struct {
	PublicationID string            `json:"publication_id,omitempty"`
	State         string            `json:"state"`
	Groups        []GroupView       `json:"groups,omitempty"`
	History       []CaseHistoryView `json:"history,omitempty"`
	NextCursor    string            `json:"next_cursor,omitempty"`
	HasMore       bool              `json:"has_more"`
}

type GroupView struct {
	ActionViewID            string       `json:"action_view_id"`
	DisplayOrder            int          `json:"display_order"`
	DisplayTitle            string       `json:"display_title"`
	DisplaySummary          string       `json:"display_summary"`
	ActionType              string       `json:"action_type"`
	Family                  string       `json:"family"`
	PresentationTemplateKey string       `json:"presentation_template_key"`
	ActiveMemberCount       int          `json:"active_member_count"`
	Members                 []MemberView `json:"members"`
	UserStateSummary        string       `json:"user_state_summary"`
	Limitations             []string     `json:"limitations,omitempty"`
}

type MemberView struct {
	CaseID            string         `json:"case_id"`
	MemberOrder       int            `json:"member_order"`
	ActionType        string         `json:"action_type"`
	TargetKind        string         `json:"target_kind"`
	TargetID          string         `json:"target_id"`
	UserState         string         `json:"user_state"`
	SystemState       string         `json:"system_state"`
	Active            bool           `json:"active"`
	Parameters        map[string]any `json:"parameters,omitempty"`
	RecommendationRef string         `json:"recommendation_ref,omitempty"`
}

type CaseHistoryView struct {
	ActionViewID            string         `json:"action_view_id"`
	CaseID                  string         `json:"case_id"`
	DisplayTitle            string         `json:"display_title"`
	DisplaySummary          string         `json:"display_summary"`
	ActionType              string         `json:"action_type"`
	Family                  string         `json:"family"`
	PresentationTemplateKey string         `json:"presentation_template_key"`
	UserFacingMode          string         `json:"user_facing_mode"`
	TargetKind              string         `json:"target_kind"`
	TargetID                string         `json:"target_id"`
	Parameters              map[string]any `json:"parameters,omitempty"`
	RecommendationRef       string         `json:"recommendation_ref,omitempty"`
}

type DetailResponse struct {
	Kind    string           `json:"kind"`
	Group   *GroupView       `json:"group,omitempty"`
	History *CaseHistoryView `json:"history,omitempty"`
}

type BoundCaseContext struct {
	CaseID                   string
	TargetKind               string
	TargetID                 string
	ActionType               string
	UserState                string
	SystemState              string
	Active                   bool
	OriginParameters         map[string]any
	OriginRecommendationRef  string
	OriginPublicationID      int64
	CurrentParameters        map[string]any
	CurrentRecommendationRef string
	CurrentPublicationID     int64
}

type MutationResponse struct {
	CaseID    string `json:"case_id"`
	UserState string `json:"user_state"`
}

type BulkRequest struct {
	CaseIDs   []string `json:"case_ids"`
	EventType string   `json:"event_type"`
}

type ListFilter struct {
	Status string
	Family string
	Query  string
	Cursor string
	Limit  int
}
