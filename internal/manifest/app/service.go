package app

import (
	"fmt"
	"os"

	"github.com/alexisjcarr/scm/internal/manifest/domain"
	"github.com/alexisjcarr/scm/internal/manifest/infra"
)

// Service owns local manifest loading and validation.
type Service struct{}

// LoadFile reads, parses, and validates a manifest from disk.
func (Service) LoadFile(path string) (domain.CompiledManifest, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.CompiledManifest{}, nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	compiled, err := Service{}.ParseAndValidate(data)
	if err != nil {
		return domain.CompiledManifest{}, nil, err
	}
	return compiled, data, nil
}

// ParseAndValidate reads the YAML DSL and returns the compiled manifest.
func (Service) ParseAndValidate(data []byte) (domain.CompiledManifest, error) {
	manifest, err := infra.DecodeManifest(data)
	if err != nil {
		return domain.CompiledManifest{}, err
	}
	return manifest.Validate()
}
