package prompt

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

// hashSeparator sits between instructions and input so that, e.g.,
// instructions="a\n" + input="---\nb" cannot collide with
// instructions="a" + input="\n---\nb".
const hashSeparator = "\n---\n"

// Hash returns the hex-encoded SHA256 digest of instructions + separator +
// input after canonical LF line-ending normalization. Callers that load
// templates via TemplateByVersion already normalize; Hash still normalizes so
// direct callers with CRLF payloads match the immutable LF identity.
func Hash(instructions, input string) string {
	instructions = lineendings.NormalizeLineEndings(instructions)
	input = lineendings.NormalizeLineEndings(input)
	sum := sha256.Sum256([]byte(instructions + hashSeparator + input))
	return hex.EncodeToString(sum[:])
}

// AnalystV1Hash returns the hash of the current production analysis template.
func AnalystV1Hash() string {
	h, err := HashForVersion(CurrentAnalystVersion)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_22Hash returns the frozen hash of the published v1_22 template.
func AnalystV1_22Hash() string {
	h, err := HashForVersion(VersionAnalystV1_22)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_24Hash returns the frozen hash of the published v1_24 template.
func AnalystV1_24Hash() string {
	h, err := HashForVersion(VersionAnalystV1_24)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_25Hash returns the frozen hash of the published v1_25 template.
func AnalystV1_25Hash() string {
	h, err := HashForVersion(VersionAnalystV1_25)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_26Hash returns the frozen hash of the published v1_26 template.
func AnalystV1_26Hash() string {
	h, err := HashForVersion(VersionAnalystV1_26)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_27Hash returns the frozen hash of the published v1_27 template.
func AnalystV1_27Hash() string {
	h, err := HashForVersion(VersionAnalystV1_27)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_28Hash returns the frozen hash of the published v1_28 template.
func AnalystV1_28Hash() string {
	h, err := HashForVersion(VersionAnalystV1_28)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_29Hash returns the frozen hash of the published v1_29 template.
func AnalystV1_29Hash() string {
	h, err := HashForVersion(VersionAnalystV1_29)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_30Hash returns the frozen hash of the published v1_30 template.
func AnalystV1_30Hash() string {
	h, err := HashForVersion(VersionAnalystV1_30)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_31Hash returns the frozen hash of the published v1_31 template.
func AnalystV1_31Hash() string {
	h, err := HashForVersion(VersionAnalystV1_31)
	if err != nil {
		panic(err)
	}
	return h
}

// AnalystV1_32Hash returns the hash of the registered (not-current) v1_32 template.
func AnalystV1_32Hash() string {
	h, err := HashForVersion(VersionAnalystV1_32)
	if err != nil {
		panic(err)
	}
	return h
}

// Historical hash helpers below keep call sites compiling. They intentionally
// do NOT claim equivalence to the current body — prefer HashForVersion /
// AnalystV1_22Hash / AnalystV1_24Hash / AnalystV1_25Hash / AnalystV1_26Hash /
// AnalystV1_27Hash / AnalystV1_28Hash / AnalystV1_29Hash / AnalystV1_30Hash /
// AnalystV1_31Hash for new code.

func AnalystV1_1Hash() string  { return AnalystV1Hash() }
func AnalystV1_2Hash() string  { return AnalystV1Hash() }
func AnalystV1_3Hash() string  { return AnalystV1Hash() }
func AnalystV1_4Hash() string  { return AnalystV1Hash() }
func AnalystV1_5Hash() string  { return AnalystV1Hash() }
func AnalystV1_6Hash() string  { return AnalystV1Hash() }
func AnalystV1_7Hash() string  { return AnalystV1Hash() }
func AnalystV1_8Hash() string  { return AnalystV1Hash() }
func AnalystV1_9Hash() string  { return AnalystV1Hash() }
func AnalystV1_10Hash() string { return AnalystV1Hash() }
func AnalystV1_11Hash() string { return AnalystV1Hash() }
func AnalystV1_12Hash() string { return AnalystV1Hash() }
func AnalystV1_13Hash() string { return AnalystV1Hash() }
func AnalystV1_14Hash() string { return AnalystV1Hash() }
func AnalystV1_15Hash() string { return AnalystV1Hash() }
func AnalystV1_16Hash() string { return AnalystV1Hash() }
func AnalystV1_17Hash() string { return AnalystV1Hash() }
func AnalystV1_18Hash() string { return AnalystV1Hash() }
func AnalystV1_19Hash() string { return AnalystV1Hash() }
func AnalystV1_20Hash() string { return AnalystV1Hash() }
func AnalystV1_21Hash() string { return AnalystV1Hash() }

// AnalystV1_23Hash is unavailable: v1_23 was never published. Callers must
// use HashForVersion and handle the error.
func AnalystV1_23Hash() string {
	panic("prompt: unified_analyst_v1_23 was never published; use HashForVersion")
}
