package lineendings

import (
	"bytes"
	"testing"
)

func TestNormalizeLineEndings_CRLFAndBareCR(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "lf unchanged", in: "a\nb\n", want: "a\nb\n"},
		{name: "crlf", in: "a\r\nb\r\n", want: "a\nb\n"},
		{name: "bare cr", in: "a\rb\r", want: "a\nb\n"},
		{name: "mixed", in: "a\r\nb\rc\n", want: "a\nb\nc\n"},
		{name: "empty", in: "", want: ""},
		{name: "no newline", in: "abc", want: "abc"},
		{name: "preserves spaces", in: "a  b\r\n\tc", want: "a  b\n\tc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeLineEndings(tc.in); got != tc.want {
				t.Fatalf("NormalizeLineEndings(%q)=%q want %q", tc.in, got, tc.want)
			}
			gotBytes := NormalizeLineEndingsBytes([]byte(tc.in))
			if !bytes.Equal(gotBytes, []byte(tc.want)) {
				t.Fatalf("NormalizeLineEndingsBytes mismatch: %q vs %q", gotBytes, tc.want)
			}
		})
	}
}

func TestNormalizeLineEndingsBytes_DoesNotMutateInput(t *testing.T) {
	in := []byte("a\r\nb")
	_ = NormalizeLineEndingsBytes(in)
	if !bytes.Equal(in, []byte("a\r\nb")) {
		t.Fatal("input slice was mutated")
	}
}
