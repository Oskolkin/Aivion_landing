package factualapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

type TokenSigner struct{ key []byte }

func NewTokenSigner(secret []byte) (TokenSigner, error) {
	if len(secret) < 32 {
		return TokenSigner{}, NewError(CodeInvalidArgument, "token signing secret must be at least 32 bytes")
	}
	return TokenSigner{append([]byte(nil), secret...)}, nil
}

type PageCursor struct {
	API, ContextHash, Method, FilterHash, Ordering string
	UniverseID                                     string
	LastID                                         int64
}
type factToken struct {
	API, ContextHash string
	UniverseID       string
	Input            TraceInput
}

func (s TokenSigner) encode(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s TokenSigner) decode(token string, v any, code ErrorCode) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return NewError(code, "malformed signed token")
	}
	b, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return NewError(code, "malformed signed token")
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return NewError(code, "malformed signed token")
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(b)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return NewError(code, "token signature invalid")
	}
	if json.Unmarshal(b, v) != nil {
		return NewError(code, "token payload invalid")
	}
	return nil
}
func (s TokenSigner) PageToken(c PageCursor) (string, error) { return s.encode(c) }
func (s TokenSigner) ParsePageToken(token string, ctx FactualReadContext, method, filterHash string) (PageCursor, error) {
	var c PageCursor
	if e := s.decode(token, &c, CodePageTokenInvalid); e != nil {
		return c, e
	}
	if c.ContextHash != ctx.ContextHash() || c.UniverseID != ctx.UniverseID() {
		return c, NewError(CodePageTokenContextMismatch, "page token belongs to another context")
	}
	if c.API != ctx.PolicyVersions().API || c.Ordering != "id_asc_v1" {
		return c, NewError(CodePageTokenInvalid, "page token contract is unsupported")
	}
	if c.Method != method || c.FilterHash != filterHash {
		return c, NewError(CodePageTokenFilterMismatch, "page token belongs to another query")
	}
	return c, nil
}
func (s TokenSigner) FactRef(ctx FactualReadContext, input TraceInput) (string, error) {
	return s.encode(factToken{API: ctx.PolicyVersions().API, ContextHash: ctx.ContextHash(), UniverseID: ctx.UniverseID(), Input: input})
}
func (s TokenSigner) ParseFactRef(token string, ctx FactualReadContext) (TraceInput, error) {
	var f factToken
	if e := s.decode(token, &f, CodeFactRefInvalid); e != nil {
		return TraceInput{}, e
	}
	if f.ContextHash != ctx.ContextHash() || f.UniverseID != ctx.UniverseID() {
		return TraceInput{}, NewError(CodeFactRefContextMismatch, "fact reference belongs to another context")
	}
	if f.API != ctx.PolicyVersions().API || f.Input.Kind == "" || f.Input.ResultHash == "" {
		return TraceInput{}, NewError(CodeFactRefInvalid, "fact reference contract is unsupported")
	}
	return f.Input, nil
}
