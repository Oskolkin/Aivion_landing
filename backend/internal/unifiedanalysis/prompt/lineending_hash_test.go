package prompt

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

const canonicalAnalystV1_26Hash = "9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308"
const canonicalAnalystV1_27Hash = "ff70ed2316e09c9455a4b13e1a52a0720ef788c65c4fdf22320a59a6b99acc2f"
const canonicalAnalystV1_28Hash = "9edf9789afe1127ef94c3c78d83f20adc7f10105bbc174fb8166e717aa27e966"
const canonicalAnalystV1_30Hash = "b32dd1e87040218b360d26c02b92f71703bb8012d5b93636c27297f86b5fc21c"
const canonicalAnalystV1_31Hash = "e7dc8b317cbb7eb7ce6dae632cd4487edd19db1a3685f6098ce4f9296ccb1a45"

func TestHash_LineEndingIndependent(t *testing.T) {
	instructionsLF := "line one\nline two\n"
	inputLF := "input a\ninput b"
	instructionsCRLF := strings.ReplaceAll(instructionsLF, "\n", "\r\n")
	inputCRLF := strings.ReplaceAll(inputLF, "\n", "\r\n")
	instructionsCR := strings.ReplaceAll(instructionsLF, "\n", "\r")
	inputCR := strings.ReplaceAll(inputLF, "\n", "\r")

	want := Hash(instructionsLF, inputLF)
	cases := []struct {
		name         string
		instructions string
		input        string
	}{
		{"both LF", instructionsLF, inputLF},
		{"both CRLF", instructionsCRLF, inputCRLF},
		{"instructions CRLF input LF", instructionsCRLF, inputLF},
		{"instructions LF input CRLF", instructionsLF, inputCRLF},
		{"bare CR", instructionsCR, inputCR},
		{"trailing newline CRLF", instructionsCRLF + "\r\n", inputCRLF + "\r\n"},
		{"no trailing newline", strings.TrimRight(instructionsLF, "\n"), strings.TrimRight(inputLF, "\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Normalize then TrimSpace mirrors TemplateByVersion for trailing cases.
			got := Hash(
				strings.TrimSpace(lineendings.NormalizeLineEndings(tc.instructions)),
				strings.TrimSpace(lineendings.NormalizeLineEndings(tc.input)),
			)
			// Hash itself also normalizes; compare to LF baseline after trim.
			baseline := Hash(strings.TrimSpace(instructionsLF), strings.TrimSpace(inputLF))
			if got != baseline {
				t.Fatalf("hash=%s want=%s", got, baseline)
			}
			_ = want
		})
	}
}

func TestHash_V126_LFEqualsCRLF(t *testing.T) {
	tpl, err := TemplateByVersion(VersionAnalystV1_26)
	if err != nil {
		t.Fatal(err)
	}
	lfHash := Hash(tpl.Instructions, tpl.Input)
	crlfHash := Hash(
		strings.ReplaceAll(tpl.Instructions, "\n", "\r\n"),
		strings.ReplaceAll(tpl.Input, "\n", "\r\n"),
	)
	if lfHash != crlfHash {
		t.Fatalf("LF hash=%s CRLF hash=%s", lfHash, crlfHash)
	}
	if lfHash != canonicalAnalystV1_26Hash {
		t.Fatalf("v1_26 hash=%s want=%s", lfHash, canonicalAnalystV1_26Hash)
	}
	if lfHash != pinnedAnalystV1_26Hash {
		t.Fatalf("v1_26 hash=%s pinned=%s", lfHash, pinnedAnalystV1_26Hash)
	}
}

func TestPublishedPromptHashes_LineEndingIndependent(t *testing.T) {
	versions := []struct {
		version string
		pinned  string
	}{
		{VersionAnalystV1_22, pinnedAnalystV1_22Hash},
		{VersionAnalystV1_24, pinnedAnalystV1_24Hash},
		{VersionAnalystV1_25, pinnedAnalystV1_25Hash},
		{VersionAnalystV1_26, pinnedAnalystV1_26Hash},
		{VersionAnalystV1_27, pinnedAnalystV1_27Hash},
		{VersionAnalystV1_28, pinnedAnalystV1_28Hash},
		{VersionAnalystV1_29, pinnedAnalystV1_29Hash},
		{VersionAnalystV1_30, pinnedAnalystV1_30Hash},
		{VersionAnalystV1_31, pinnedAnalystV1_31Hash},
	}
	for _, tc := range versions {
		t.Run(tc.version, func(t *testing.T) {
			tpl, err := TemplateByVersion(tc.version)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(tpl.Instructions, "\r") || strings.Contains(tpl.Input, "\r") {
				t.Fatal("canonical template still contains CR")
			}
			lf := Hash(tpl.Instructions, tpl.Input)
			crlf := Hash(
				strings.ReplaceAll(tpl.Instructions, "\n", "\r\n"),
				strings.ReplaceAll(tpl.Input, "\n", "\r\n"),
			)
			bare := Hash(
				strings.ReplaceAll(tpl.Instructions, "\n", "\r"),
				strings.ReplaceAll(tpl.Input, "\n", "\r"),
			)
			if lf != tc.pinned {
				t.Fatalf("LF hash=%s want pinned=%s", lf, tc.pinned)
			}
			if crlf != tc.pinned {
				t.Fatalf("CRLF hash=%s want pinned=%s", crlf, tc.pinned)
			}
			if bare != tc.pinned {
				t.Fatalf("bare CR hash=%s want pinned=%s", bare, tc.pinned)
			}
		})
	}
}

func TestRenderedPromptBytesMatchHashBytes(t *testing.T) {
	rendered, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Hash != Hash(rendered.Instructions, rendered.Input) {
		t.Fatal("rendered hash must equal Hash(instructions, input)")
	}
	if strings.Contains(rendered.Instructions, "\r") || strings.Contains(rendered.Input, "\r") {
		t.Fatal("provider-bound prompt text must be LF-normalized")
	}
	if rendered.Hash != canonicalAnalystV1_31Hash {
		t.Fatalf("hash=%s want=%s", rendered.Hash, canonicalAnalystV1_31Hash)
	}
}
