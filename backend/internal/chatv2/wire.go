package chatv2

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewFromPool(pool *pgxpool.Pool, hmacSecret string, pubs PublicationReader, actions ActionViews, latest LatestStableResolver, provider ChatAgentProvider, artifacts ArtifactStore) (*Service, error) {
	secret := strings.TrimSpace(hmacSecret)
	if len(secret) < 32 {
		secret = strings.Repeat("c", 32)
	}
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("factual token signer: %w", err)
	}
	store := NewStore(pool)
	api := factual.NewService(pool, signer, 0, 0)
	return &Service{
		Store: store,
		Resolver: &ContextResolver{
			Pubs: pubs, Latest: latest, Actions: actions, Memory: DefaultMemoryPolicy(), Sessions: store,
		},
		Agent: &Agent{
			Provider: provider,
			Tools:    &ToolExecutor{API: api, Pubs: pubs, Policy: DefaultToolPolicy()},
			Policy:   DefaultToolPolicy(),
			Retry:    DefaultRetryPolicy(),
		},
		Actions:  actions,
		Exporter: &FactualExporter{Store: store, Artifacts: artifacts, Resolver: &DatasetResolver{API: api}},
	}, nil
}
