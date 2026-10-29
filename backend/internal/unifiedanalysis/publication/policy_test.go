package publication

import "testing"

func TestPublicationPolicy_Allows_AcceptsCurrentPromptAndSchema(t *testing.T) {
	policy := DefaultPublicationPolicy()

	err := policy.Allows(policy.AcceptedPromptVersion, policy.AcceptedPromptHash, policy.AcceptedSchemaVersion, policy.AcceptedSchemaHash, false)
	if err != nil {
		t.Fatalf("expected accepted prompt/schema identity to pass policy, got: %v", err)
	}
}

func TestPublicationPolicy_Allows_RejectsWrongPromptVersion(t *testing.T) {
	policy := DefaultPublicationPolicy()

	err := policy.Allows("some_other_prompt_v1", policy.AcceptedPromptHash, policy.AcceptedSchemaVersion, policy.AcceptedSchemaHash, false)
	if err == nil {
		t.Fatal("expected error for mismatched prompt version")
	}
	code, ok := CodeOf(err)
	if !ok || code != CodePromptNotAllowed {
		t.Fatalf("expected error code %q, got %q (ok=%v)", CodePromptNotAllowed, code, ok)
	}
}

func TestPublicationPolicy_Allows_RejectsWrongSchemaVersion(t *testing.T) {
	policy := DefaultPublicationPolicy()

	err := policy.Allows(policy.AcceptedPromptVersion, policy.AcceptedPromptHash, "some_other_schema_v1", policy.AcceptedSchemaHash, false)
	if err == nil {
		t.Fatal("expected error for mismatched schema version")
	}
	code, ok := CodeOf(err)
	if !ok || code != CodeSchemaNotAllowed {
		t.Fatalf("expected error code %q, got %q (ok=%v)", CodeSchemaNotAllowed, code, ok)
	}
}

func TestPublicationPolicy_Allows_RejectsWrongPromptHash(t *testing.T) {
	policy := DefaultPublicationPolicy()

	err := policy.Allows(policy.AcceptedPromptVersion, "sha256:not-the-real-hash", policy.AcceptedSchemaVersion, policy.AcceptedSchemaHash, false)
	if err == nil {
		t.Fatal("expected error for mismatched prompt hash")
	}
	code, ok := CodeOf(err)
	if !ok || code != CodePromptNotAllowed {
		t.Fatalf("expected error code %q, got %q (ok=%v)", CodePromptNotAllowed, code, ok)
	}
}

func TestPublicationPolicy_Allows_RejectsMissingIdentifiers(t *testing.T) {
	policy := DefaultPublicationPolicy()

	if err := policy.Allows("", policy.AcceptedPromptHash, policy.AcceptedSchemaVersion, policy.AcceptedSchemaHash, false); err == nil {
		t.Fatal("expected error for empty prompt version")
	}
	if err := policy.Allows(policy.AcceptedPromptVersion, policy.AcceptedPromptHash, "", policy.AcceptedSchemaHash, false); err == nil {
		t.Fatal("expected error for empty schema version")
	}
}

func TestPublicationPolicy_Allows_AllowNonCurrentBypassesMismatch(t *testing.T) {
	policy := DefaultPublicationPolicy()

	err := policy.Allows("historical_prompt_v0", "sha256:old", "historical_schema_v0", "sha256:old", true)
	if err != nil {
		t.Fatalf("expected allowNonCurrent=true to bypass mismatch, got: %v", err)
	}
}

func TestDefaultPublicationPolicy_HasStableAdapterVersion(t *testing.T) {
	policy := DefaultPublicationPolicy()
	if policy.AdapterVersion != AdapterVersion {
		t.Fatalf("expected adapter version %q, got %q", AdapterVersion, policy.AdapterVersion)
	}
	if policy.AcceptedPromptVersion == "" || policy.AcceptedSchemaVersion == "" {
		t.Fatal("expected non-empty accepted prompt/schema versions")
	}
}
