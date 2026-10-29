package pack

import "errors"

// Sentinel errors returned by the Unified Analysis Pack builder, validator and service.
var (
	// ErrNilSnapshot is returned when Build is called with a nil AnalysisDataSnapshot.
	ErrNilSnapshot = errors.New("unifiedanalysis/pack: snapshot must not be nil")
	// ErrInvalidSellerAccountID is returned when seller_account_id is not positive.
	ErrInvalidSellerAccountID = errors.New("unifiedanalysis/pack: seller_account_id must be > 0")
	// ErrEmptyZip is returned by the validator when the ZIP payload is empty.
	ErrEmptyZip = errors.New("unifiedanalysis/pack: zip payload is empty")
	// ErrValidationFailed is returned when a built pack fails technical validation.
	ErrValidationFailed = errors.New("unifiedanalysis/pack: pack failed technical validation")
	// ErrBusinessConsistencyFailed is returned when business consistency errors > 0.
	ErrBusinessConsistencyFailed = errors.New("unifiedanalysis/pack: pack failed business consistency validation")
	// ErrPackTooLarge is returned when the compressed or uncompressed pack size exceeds limits.
	ErrPackTooLarge = errors.New("unifiedanalysis/pack: pack exceeds configured size limits")
	// ErrTooManyRows is returned when total CSV row count exceeds the configured limit.
	ErrTooManyRows = errors.New("unifiedanalysis/pack: pack exceeds configured row limit")
	// ErrServiceNotConfigured is returned when Service is used without required dependencies.
	ErrServiceNotConfigured = errors.New("unifiedanalysis/pack: service is not configured")
	// ErrStorageNotConfigured is returned when BuildAndStore is called without a storage backend.
	ErrStorageNotConfigured = errors.New("unifiedanalysis/pack: storage is not configured")
)
