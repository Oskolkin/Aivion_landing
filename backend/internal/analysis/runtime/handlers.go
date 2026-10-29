package runtime

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
)

type ResolveContextHandler struct {
	Resolve func(context.Context, Work) ([]byte, error)
}

func (h ResolveContextHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Resolve == nil {
		return Output{}, fmt.Errorf("runtime: RESOLVE_CONTEXT not configured")
	}
	body, err := h.Resolve(ctx, w)
	return Output{Kind: artifacts.ResolvedContextManifest, Body: body}, err
}

type SealUniverseHandler struct {
	Seal func(context.Context, Work) ([]byte, error)
}

func (h SealUniverseHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Seal == nil {
		return Output{}, fmt.Errorf("runtime: SEAL_UNIVERSE not configured")
	}
	body, err := h.Seal(ctx, w)
	return Output{Kind: artifacts.FactualUniverseManifest, Body: body}, err
}

type ContinuityHandler struct {
	Prepare func(context.Context, Work) ([]byte, error)
}

func (h ContinuityHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Prepare == nil {
		return Output{}, fmt.Errorf("runtime: PREPARE_CONTINUITY not configured")
	}
	body, err := h.Prepare(ctx, w)
	return Output{Kind: artifacts.ContinuityContext, Body: body}, err
}

type LandscapeCoreHandler struct {
	Build func(context.Context, Work) ([]byte, error)
}

func (h LandscapeCoreHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Build == nil {
		return Output{}, fmt.Errorf("runtime: BUILD_LANDSCAPE_CORE not configured")
	}
	body, err := h.Build(ctx, w)
	return Output{Kind: artifacts.LandscapeCoreSet, Body: body}, err
}

type MaterialityHandler struct {
	Run func(context.Context, Work) ([]byte, error)
}

func (h MaterialityHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Run == nil {
		return Output{}, fmt.Errorf("runtime: RUN_MATERIALITY not configured")
	}
	body, err := h.Run(ctx, w)
	return Output{Kind: artifacts.MaterialitySelection, Body: body}, err
}

type LandscapeDetailHandler struct {
	Build func(context.Context, Work) ([]byte, error)
}

func (h LandscapeDetailHandler) Execute(ctx context.Context, w Work) (Output, error) {
	if h.Build == nil {
		return Output{}, fmt.Errorf("runtime: BUILD_LANDSCAPE_DETAIL not configured")
	}
	body, err := h.Build(ctx, w)
	return Output{Kind: artifacts.LandscapeDetailSet, Body: body}, err
}
