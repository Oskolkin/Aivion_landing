package openaix_test

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestDefaultRequestTimeoutSeconds(t *testing.T) {
	if openaix.DefaultRequestTimeoutSeconds != 120 {
		t.Fatalf("DefaultRequestTimeoutSeconds=%d want 120", openaix.DefaultRequestTimeoutSeconds)
	}
	if openaix.DefaultRequestTimeout != 120*time.Second {
		t.Fatalf("DefaultRequestTimeout=%s", openaix.DefaultRequestTimeout)
	}
}

func TestBuildHTTPClientDefaultTimeout(t *testing.T) {
	client, err := openaix.BuildHTTPClient(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if client.Timeout != openaix.DefaultRequestTimeout {
		t.Fatalf("timeout=%s want %s", client.Timeout, openaix.DefaultRequestTimeout)
	}
}

func TestBuildHTTPClientOverrideTimeout(t *testing.T) {
	client, err := openaix.BuildHTTPClient(45, "")
	if err != nil {
		t.Fatal(err)
	}
	if client.Timeout != 45*time.Second {
		t.Fatalf("timeout=%s", client.Timeout)
	}
}

func TestDefaultGlobalContextTimeoutSeconds(t *testing.T) {
	if openaix.DefaultGlobalContextTimeoutSeconds != 300 {
		t.Fatalf("DefaultGlobalContextTimeoutSeconds=%d want 300", openaix.DefaultGlobalContextTimeoutSeconds)
	}
	if openaix.DefaultGlobalContextTimeout != 300*time.Second {
		t.Fatalf("DefaultGlobalContextTimeout=%s", openaix.DefaultGlobalContextTimeout)
	}
}

func TestDefaultActionsTimeoutSeconds(t *testing.T) {
	if openaix.DefaultActionsTimeoutSeconds != 900 {
		t.Fatalf("DefaultActionsTimeoutSeconds=%d want 900", openaix.DefaultActionsTimeoutSeconds)
	}
	if openaix.DefaultActionsTimeout != 900*time.Second {
		t.Fatalf("DefaultActionsTimeout=%s", openaix.DefaultActionsTimeout)
	}
}
