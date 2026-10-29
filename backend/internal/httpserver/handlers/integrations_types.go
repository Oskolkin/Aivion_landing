package handlers

type accountResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ozonConnectionResponse struct {
	ID              int64   `json:"id"`
	SellerAccountID int64   `json:"seller_account_id"`
	Status          string  `json:"status"`
	LastCheckAt     *string `json:"last_check_at"`
	LastCheckResult *string `json:"last_check_result"`
	LastError       *string `json:"last_error"`
	HasCredentials  bool    `json:"has_credentials"`
	ClientIDMasked  string  `json:"client_id_masked"`

	PerformanceTokenSet        bool    `json:"performance_token_set"`
	PerformanceStatus          string  `json:"performance_status"`
	PerformanceLastCheckAt     *string `json:"performance_last_check_at"`
	PerformanceLastCheckResult *string `json:"performance_last_check_result"`
	PerformanceLastError       *string `json:"performance_last_error"`
}

type upsertOzonConnectionRequest struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
	// Optional on create; for updates use PUT /performance-token.
	PerformanceBearerToken *string `json:"performance_bearer_token,omitempty"`
}

type ozonCheckResponse struct {
	Status    string  `json:"status"`
	CheckedAt string  `json:"checked_at"`
	Message   string  `json:"message"`
	ErrorCode *string `json:"error_code"`
}
