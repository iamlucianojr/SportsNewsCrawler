package transformer

import (
	"fmt"

	"github.com/SportsNewsCrawler/internal/domain"
)

// GetTransformer returns the appropriate transformer by name.
// This acts as a factory/registry.
func GetTransformer(name string) (domain.Transformer, error) {
	switch name {
	case "pulselive":
		return NewPulseLiveTransformer(), nil
	case "dummy":
		return NewDummyTransformer(), nil
	default:
		return nil, fmt.Errorf("transformer not found: %s", name)
	}
}
