package chatextract

import (
	"os"
	"strconv"
)

const (
	defaultMaxInputBytes          int64 = 50 << 20
	defaultZipMaxFiles                  = 50
	defaultZipMaxUncompressed     int64 = 50 << 20
	defaultZipMaxCompressionRatio       = 100
)

type limits struct {
	maxInputBytes          int64
	zipMaxFiles            int
	zipMaxUncompressed     int64
	zipMaxCompressionRatio int64
}

func configuredLimits() limits {
	return limits{
		maxInputBytes:          envInt64("CHATEXTRACT_MAX_INPUT_BYTES", defaultMaxInputBytes),
		zipMaxFiles:            int(envInt64("CHATEXTRACT_ZIP_MAX_FILES", defaultZipMaxFiles)),
		zipMaxUncompressed:     envInt64("CHATEXTRACT_ZIP_MAX_UNCOMPRESSED_BYTES", defaultZipMaxUncompressed),
		zipMaxCompressionRatio: envInt64("CHATEXTRACT_ZIP_MAX_COMPRESSION_RATIO", defaultZipMaxCompressionRatio),
	}
}

func envInt64(name string, fallback int64) int64 {
	value, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
