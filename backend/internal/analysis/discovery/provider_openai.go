package discovery

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type OpenAIProvider struct {
	Client *openaix.ResponsesClient
	Model  string
}

func (p *OpenAIProvider) Discover(ctx context.Context, req Request) (Response, error) {
	if p == nil || p.Client == nil {
		return Response{}, packErr(CodeProviderNotConfigured, "openai client missing")
	}
	model := req.Model
	if model == "" {
		model = p.Model
	}
	start := time.Now()
	wire := req
	wire.Model = model
	out, err := p.Client.Create(ctx, BuildDiscoveryResponsesRequest(wire))
	lat := time.Since(start)
	if err != nil {
		return Response{}, mapOpenAIError(err)
	}
	resp := Response{
		Status: out.Status, OutputText: out.OutputText, Refusal: out.Refusal,
		IncompleteReason: out.IncompleteReason, ProviderRequestID: out.RequestID,
		Latency: lat, RawJSON: out.Raw,
		Usage: Usage{
			InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens,
			ReasoningTokens: out.Usage.ReasoningTokens, CachedInputTokens: out.Usage.CachedInputTokens,
			TotalTokens: out.Usage.TotalTokens,
		},
	}
	return resp, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func mapOpenAIError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return timeoutErr(err.Error())
	}
	var httpErr *openaix.HTTPOutageError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusTooManyRequests:
			return transientErr(CodeProviderRateLimit, err.Error())
		case http.StatusUnauthorized, http.StatusForbidden:
			return &Error{Code: CodeProviderAuth, Msg: err.Error(), class: policy.InternalBug}
		case http.StatusBadRequest:
			return packErr(CodeProviderBadRequest, err.Error())
		}
		if httpErr.StatusCode >= 500 {
			return transientErr(CodeProviderServerError, err.Error())
		}
	}
	if openaix.IsTemporarilyUnavailable(err) {
		return transientErr(CodeProviderServerError, err.Error())
	}
	return transientErr(CodeProviderInvalidResponse, err.Error())
}
