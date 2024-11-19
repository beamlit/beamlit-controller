package proxy

import (
	"context"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
	"golang.org/x/exp/rand"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type weightedBackend struct {
	backend v1alpha1.Backend
	weight  int
}

func weightedRandomBackend(_ context.Context, backends []weightedBackend, totalWeight int) v1alpha1.Backend {
	randomIndex := rand.Intn(totalWeight)
	lastWeight := 0
	for _, weightedBackend := range backends {
		lastWeight += weightedBackend.weight
		if randomIndex < lastWeight {
			return weightedBackend.backend
		}
	}
	return backends[len(backends)-1].backend
}

func handleOAuth(ctx context.Context, auth *v1alpha1.Auth) (*oauth2.Token, error) {
	if auth.OAuth == nil {
		return nil, nil
	}
	oauth := auth.OAuth
	cfg := clientcredentials.Config{
		ClientID:     oauth.ClientID,
		ClientSecret: oauth.ClientSecret,
		TokenURL:     oauth.TokenURL,
	}
	token, err := cfg.Token(ctx)
	if err != nil {
		return nil, err
	}
	return token, nil
}
