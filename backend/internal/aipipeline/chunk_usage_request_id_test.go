package aipipeline

import "testing"

func TestBuildParseChunkUsageRequestID(t *testing.T) {
	tests := []struct {
		name       string
		chunkID    string
		providerID string
		wantChunk  string
		wantProv   string
		wantEnc    bool
	}{
		{
			name:       "full",
			chunkID:    "stock_positions_001",
			providerID: "req_abc",
			wantChunk:  "stock_positions_001",
			wantProv:   "req_abc",
			wantEnc:    true,
		},
		{
			name:       "empty provider",
			chunkID:    "stock_positions_001",
			providerID: "",
			wantChunk:  "stock_positions_001",
			wantProv:   "",
			wantEnc:    true,
		},
		{
			name:       "empty chunk",
			chunkID:    "",
			providerID: "req_only",
			wantChunk:  "",
			wantProv:   "req_only",
			wantEnc:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := BuildChunkUsageRequestID(tc.chunkID, tc.providerID)
			parsed := ParseChunkUsageRequestID(encoded)
			if parsed.Encoded != tc.wantEnc {
				t.Fatalf("encoded=%v want %v (%q)", parsed.Encoded, tc.wantEnc, encoded)
			}
			if parsed.ChunkID != tc.wantChunk {
				t.Fatalf("chunk_id=%q want %q", parsed.ChunkID, tc.wantChunk)
			}
			if parsed.ProviderRequestID != tc.wantProv {
				t.Fatalf("provider_request_id=%q want %q", parsed.ProviderRequestID, tc.wantProv)
			}
			rebuilt := BuildChunkUsageRequestID(parsed.ChunkID, parsed.ProviderRequestID)
			if tc.wantEnc && rebuilt != encoded {
				t.Fatalf("rebuilt=%q encoded=%q", rebuilt, encoded)
			}
		})
	}
}

func TestParseChunkUsageRequestID_NonChunkUnchanged(t *testing.T) {
	const plain = "req_dash_123"
	parsed := ParseChunkUsageRequestID(plain)
	if parsed.Encoded {
		t.Fatal("expected non-chunk id")
	}
	if parsed.ProviderRequestID != plain {
		t.Fatalf("provider_request_id=%q", parsed.ProviderRequestID)
	}
	if parsed.ChunkID != "" {
		t.Fatalf("chunk_id=%q", parsed.ChunkID)
	}
}
