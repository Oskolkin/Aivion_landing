package unifiedanalysis

// Run status values for unified_analysis mode.
const (
	StatusCreated                   = "created"
	StatusPackBuilding              = "pack_building"
	StatusPackValidating            = "pack_validating"
	StatusPackReady                 = "pack_ready"
	StatusProviderContainerCreating = "provider_container_creating"
	StatusProviderContainerReady    = "provider_container_ready"
	StatusProviderFileUploading     = "provider_file_uploading"
	StatusProviderFileReady         = "provider_file_ready"
	StatusProviderResponseStarting  = "provider_response_starting"
	StatusProviderRunning           = "provider_running"
	StatusProviderCompleted         = "provider_completed"
	StatusOutputValidating          = "output_validating"
	StatusPublicationReady          = "publication_ready"
	StatusPublicationBlocked        = "publication_blocked"
	StatusPublishing                = "publishing"
	StatusPublished                 = "published"
	StatusPublicationFailed         = "publication_failed"
	StatusFailed                    = "failed"
	StatusCancelled                 = "cancelled"

	StatusCleanupPending   = "cleanup_pending"
	StatusCleanupRunning   = "cleanup_running"
	StatusCleanupCompleted = "cleanup_completed"
)

// Cleanup status values (preferred over run status for post-provider cleanup).
const (
	CleanupStatusPending   = "pending"
	CleanupStatusRunning   = "running"
	CleanupStatusCompleted = "completed"
	CleanupStatusFailed    = "failed"
)

var unifiedAnalysisTransitions = map[string][]string{
	StatusCreated: {
		StatusPackBuilding,
		StatusFailed,
		StatusCancelled,
	},
	StatusPackBuilding: {
		StatusPackValidating,
		StatusFailed,
		StatusCancelled,
	},
	StatusPackValidating: {
		StatusPackReady,
		StatusFailed,
		StatusCancelled,
	},
	StatusPackReady: {
		StatusProviderContainerCreating,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderContainerCreating: {
		StatusProviderContainerReady,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderContainerReady: {
		StatusProviderFileUploading,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderFileUploading: {
		StatusProviderFileReady,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderFileReady: {
		StatusProviderResponseStarting,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderResponseStarting: {
		StatusProviderRunning,
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderRunning: {
		StatusProviderCompleted,
		StatusOutputValidating, // analysis profile may skip lingering at provider_completed
		StatusFailed,
		StatusCancelled,
	},
	StatusProviderCompleted: {
		StatusOutputValidating,
		StatusFailed,
		StatusCancelled,
	},
	StatusOutputValidating: {
		StatusPublicationReady,
		StatusPublicationBlocked,
		StatusFailed,
	},
	// Stage 5 publication: publication_ready is not hard-terminal (see
	// terminalRunStatuses below) precisely so it can advance through
	// publishing to published/publication_failed.
	StatusPublicationReady: {
		StatusPublishing,
		StatusPublished,
		StatusPublicationFailed,
	},
	StatusPublishing: {
		StatusPublished,
		StatusPublicationFailed,
	},
	StatusCleanupPending: {
		StatusCleanupRunning,
		StatusFailed,
		StatusCancelled,
	},
	StatusCleanupRunning: {
		StatusCleanupCompleted,
		StatusFailed,
	},
}

// Hard terminal statuses — no further orchestration transitions.
// publication_ready is intentionally NOT here: Stage 5 publication advances
// it to publishing -> published/publication_failed. It is still a
// wait-terminal status for polling loops (see IsWaitTerminal).
var terminalRunStatuses = map[string]struct{}{
	StatusFailed:             {},
	StatusCancelled:          {},
	StatusPublicationBlocked: {},
	StatusPublished:          {},
	StatusPublicationFailed:  {},
	StatusCleanupCompleted:   {},
}

// IsActiveUnifiedRunStatus reports statuses that block creating another run for the same seller+date.
// Must stay aligned with migration unique index exclusions.
func IsActiveUnifiedRunStatus(status string) bool {
	switch status {
	case StatusFailed, StatusCancelled, StatusProviderCompleted,
		StatusPublicationReady, StatusPublicationBlocked, StatusCleanupCompleted,
		StatusPublishing, StatusPublished, StatusPublicationFailed:
		return false
	default:
		return true
	}
}

// CanTransition reports whether from may advance to to for the given pipeline mode.
func CanTransition(mode, from, to string) bool {
	if mode != ModeUnifiedAnalysis {
		return false
	}
	if from == to {
		return true
	}
	if _, terminal := terminalRunStatuses[from]; terminal {
		return false
	}
	allowed, ok := unifiedAnalysisTransitions[from]
	if !ok {
		return false
	}
	for _, next := range allowed {
		if next == to {
			return true
		}
	}
	return false
}

// IsTerminalRunStatus reports hard-terminal statuses (publication_* / failed / cancelled).
// Smoke profile also treats provider_completed as wait-terminal via IsWaitTerminal.
func IsTerminalRunStatus(status string) bool {
	_, ok := terminalRunStatuses[status]
	return ok
}

// IsWaitTerminal reports whether queue/devtools wait loops may stop for the given profile.
func IsWaitTerminal(status string, analysisProfile bool) bool {
	switch status {
	case StatusFailed, StatusCancelled:
		return true
	case StatusPublicationReady, StatusPublicationBlocked, StatusPublished:
		return true
	case StatusProviderCompleted:
		return !analysisProfile
	default:
		return false
	}
}
