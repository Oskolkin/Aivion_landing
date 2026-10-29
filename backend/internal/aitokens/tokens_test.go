package aitokens

import "testing"

func TestEstimateBytesTokens(t *testing.T) {
	if got := EstimateBytesTokens([]byte("abcd")); got != 1 {
		t.Fatalf("got=%d want 1", got)
	}
	if got := EstimateBytesTokens(nil); got != 0 {
		t.Fatalf("got=%d want 0", got)
	}
}

func TestEstimateJSONTokens(t *testing.T) {
	est, err := EstimateJSONTokens(map[string]string{"text": "пример"})
	if err != nil {
		t.Fatal(err)
	}
	if est <= 0 {
		t.Fatalf("expected positive estimate, got %d", est)
	}
}

func TestMaxInputTokensPerRequest(t *testing.T) {
	tests := []struct {
		name       string
		perRequest string
		approx     string
		want       int
	}{
		{
			name:       "primary env valid",
			perRequest: "50000",
			approx:     "99999",
			want:       50000,
		},
		{
			name:       "primary invalid fallback valid",
			perRequest: "not-a-number",
			approx:     "64000",
			want:       64000,
		},
		{
			name:       "both absent",
			perRequest: "",
			approx:     "",
			want:       DefaultMaxInputTokensPerRequest,
		},
		{
			name:       "both invalid",
			perRequest: "bad",
			approx:     "also-bad",
			want:       DefaultMaxInputTokensPerRequest,
		},
		{
			name:       "primary zero uses approx",
			perRequest: "0",
			approx:     "88000",
			want:       88000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envMaxInputTokensPerRequest, tt.perRequest)
			t.Setenv(envMaxInputTokensApprox, tt.approx)
			if got := MaxInputTokensPerRequest(); got != tt.want {
				t.Fatalf("got=%d want=%d", got, tt.want)
			}
		})
	}
}
