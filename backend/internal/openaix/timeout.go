package openaix

import "time"

// DefaultRequestTimeoutSeconds is the pilot default for a full OpenAI Responses API
// request lifecycle (connect, proxy, headers, response body).
const DefaultRequestTimeoutSeconds = 120

// DefaultRequestTimeout is DefaultRequestTimeoutSeconds as a time.Duration.
const DefaultRequestTimeout = DefaultRequestTimeoutSeconds * time.Second

// DefaultGlobalContextTimeoutSeconds is the pilot default for Global Context synthesis.
const DefaultGlobalContextTimeoutSeconds = 300

// DefaultGlobalContextTimeout is DefaultGlobalContextTimeoutSeconds as a time.Duration.
const DefaultGlobalContextTimeout = DefaultGlobalContextTimeoutSeconds * time.Second

// DefaultActionsTimeoutSeconds is the pilot default for Actions AI First compose.
const DefaultActionsTimeoutSeconds = 900

// DefaultActionsTimeout is DefaultActionsTimeoutSeconds as a time.Duration.
const DefaultActionsTimeout = DefaultActionsTimeoutSeconds * time.Second
