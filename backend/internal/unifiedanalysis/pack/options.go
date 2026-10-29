package pack

// Default technical limits and locale settings for Unified Analysis Pack v1.
const (
	DefaultCurrency = "RUB"
	DefaultTimezone = "Europe/Moscow"

	// DefaultMaxCompressedBytes is the soft ceiling for the ZIP payload size (25 MiB).
	DefaultMaxCompressedBytes int64 = 25 * 1024 * 1024
	// DefaultMaxUncompressedBytes is the soft ceiling for total uncompressed content (100 MiB).
	DefaultMaxUncompressedBytes int64 = 100 * 1024 * 1024
	// DefaultMaxRows is the soft ceiling for total CSV data rows across all tables.
	DefaultMaxRows = 1_000_000
)

// BuildOptions configures pack building and technical validation limits.
type BuildOptions struct {
	// Currency is the ISO currency code used for money fields (RUB for v1).
	Currency string
	// Timezone is the seller-facing timezone label recorded in the manifest.
	Timezone string
	// MaxCompressedBytes bounds the total ZIP payload size. Zero uses DefaultMaxCompressedBytes.
	MaxCompressedBytes int64
	// MaxUncompressedBytes bounds total uncompressed content across all files.
	MaxUncompressedBytes int64
	// MaxRows bounds the total number of CSV data rows (excluding headers) across all tables.
	MaxRows int
}

// DefaultBuildOptions returns the frozen v1 defaults.
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		Currency:             DefaultCurrency,
		Timezone:             DefaultTimezone,
		MaxCompressedBytes:   DefaultMaxCompressedBytes,
		MaxUncompressedBytes: DefaultMaxUncompressedBytes,
		MaxRows:              DefaultMaxRows,
	}
}

// withDefaults fills zero-valued fields with v1 defaults.
func (o BuildOptions) withDefaults() BuildOptions {
	out := o
	if out.Currency == "" {
		out.Currency = DefaultCurrency
	}
	if out.Timezone == "" {
		out.Timezone = DefaultTimezone
	}
	if out.MaxCompressedBytes <= 0 {
		out.MaxCompressedBytes = DefaultMaxCompressedBytes
	}
	if out.MaxUncompressedBytes <= 0 {
		out.MaxUncompressedBytes = DefaultMaxUncompressedBytes
	}
	if out.MaxRows <= 0 {
		out.MaxRows = DefaultMaxRows
	}
	return out
}
