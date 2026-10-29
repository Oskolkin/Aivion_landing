package adsync

import (
	"context"
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type stubOzonIntegration struct {
	creds ozon.DecryptedCredentials
	cerr  error

	perfToken string
	perfErr   error
}

func (s *stubOzonIntegration) GetDecryptedCredentials(
	_ context.Context,
	_ int64,
) (ozon.DecryptedCredentials, error) {
	return s.creds, s.cerr
}

func (s *stubOzonIntegration) GetDecryptedPerformanceBearerToken(
	_ context.Context,
	_ int64,
) (string, error) {
	return s.perfToken, s.perfErr
}

func TestRun_RequiresPerformanceToken_NoSellerAPIKeyFallback(t *testing.T) {
	s := NewService(nil, &stubOzonIntegration{
		creds: ozon.DecryptedCredentials{
			ClientID: "client-id",
			APIKey:   "seller-api-key-must-not-be-used-as-bearer",
		},
		perfErr: ozon.ErrPerformanceTokenNotConfigured,
	}, nil)

	_, err := s.Run(context.Background(), RunInput{
		SellerAccountID: 1,
		ImportJobID:     1,
	})
	if !errors.Is(err, ozon.ErrPerformanceTokenNotConfigured) {
		t.Fatalf("expected ErrPerformanceTokenNotConfigured, got %v", err)
	}
}
