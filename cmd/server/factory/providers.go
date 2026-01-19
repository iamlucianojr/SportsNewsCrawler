package factory

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/provider"
	"github.com/SportsNewsCrawler/internal/infra/transformer"
	"github.com/SportsNewsCrawler/pkg/config"
)

// NewProviders creates all configured news providers.
func NewProviders(cfg *config.Config) ([]domain.Provider, error) {
	if len(cfg.Sources) == 0 {
		return nil, errors.New("no sources configured")
	}

	var providers []domain.Provider
	for _, source := range cfg.Sources {
		tr, err := transformer.GetTransformer(source.Transformer)
		if err != nil {
			slog.Warn("Skipping source", "source", source.Name, "error", err)
			continue
		}

		p := provider.NewGenericProvider(source.Name, source.URL, tr, source.Pagination)
		providers = append(providers, p)
		slog.Info("Registered provider", "provider", source.Name, "transformer", source.Transformer)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid providers configured")
	}
	return providers, nil
}
