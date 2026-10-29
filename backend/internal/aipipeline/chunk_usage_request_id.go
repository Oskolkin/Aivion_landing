package aipipeline

import "strings"

const chunkUsageRequestIDPrefix = "chunk:"

// ParsedChunkUsageRequestID describes a decoded chunk usage request key.
type ParsedChunkUsageRequestID struct {
	Encoded           bool
	ChunkID           string
	ProviderRequestID string
}

// BuildChunkUsageRequestID encodes chunk_id and provider request id for ai_usage_events.request_id.
// usage.request_id for chunk_analysis is an encoded request key; the provider request id is stored
// separately in debug artifacts and diagnostics.
func BuildChunkUsageRequestID(chunkID, providerRequestID string) string {
	chunkID = strings.TrimSpace(chunkID)
	providerRequestID = strings.TrimSpace(providerRequestID)
	if chunkID == "" {
		return providerRequestID
	}
	if providerRequestID == "" {
		return chunkUsageRequestIDPrefix + chunkID
	}
	return chunkUsageRequestIDPrefix + chunkID + ":" + providerRequestID
}

// ParseChunkUsageRequestID decodes chunk usage request keys produced by BuildChunkUsageRequestID.
// Non-chunk request ids are returned unchanged with Encoded=false.
func ParseChunkUsageRequestID(encoded string) ParsedChunkUsageRequestID {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return ParsedChunkUsageRequestID{}
	}
	if !strings.HasPrefix(encoded, chunkUsageRequestIDPrefix) {
		return ParsedChunkUsageRequestID{ProviderRequestID: encoded}
	}
	rest := strings.TrimPrefix(encoded, chunkUsageRequestIDPrefix)
	if rest == "" {
		return ParsedChunkUsageRequestID{Encoded: true}
	}
	idx := strings.Index(rest, ":")
	if idx < 0 {
		return ParsedChunkUsageRequestID{Encoded: true, ChunkID: rest}
	}
	return ParsedChunkUsageRequestID{
		Encoded:           true,
		ChunkID:           rest[:idx],
		ProviderRequestID: rest[idx+1:],
	}
}
