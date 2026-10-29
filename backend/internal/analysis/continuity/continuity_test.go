package continuity

import (
	"strings"
	"testing"
)

func TestPrepareNoBase(t *testing.T) {
	body, hash, err := Prepare(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || len(hash) != 64 {
		t.Fatalf("expected body+hash, got hash=%q len(body)=%d", hash, len(body))
	}
	zero := int64(0)
	body2, hash2, err := Prepare(&zero)
	if err != nil || hash2 != hash {
		t.Fatalf("base_publication_id=0 must equal nil: err=%v hash=%q vs %q", err, hash2, hash)
	}
	_ = body2
	if !strings.Contains(string(body), ModeNoBasePublication) {
		t.Fatalf("artifact missing mode: %s", body)
	}
	if !strings.Contains(string(body), ContractInitialNoBaseV1) {
		t.Fatalf("artifact missing contract version: %s", body)
	}
}

func TestPrepareBaseFailsClosed(t *testing.T) {
	id := int64(108)
	_, _, err := Prepare(&id)
	if err == nil || !strings.Contains(err.Error(), ErrReaderNotImplemented) {
		t.Fatalf("expected CONTINUITY_READER_NOT_IMPLEMENTED, got %v", err)
	}
}
