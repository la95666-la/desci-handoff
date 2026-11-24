// Package strategies contains per-component release implementations.
package strategies

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/joincare/jc-release/internal/shared"
)

// Strategy is the contract each component-specific upgrade path satisfies.
type Strategy interface {
	Component() shared.Component

	// Apply the release: src is the path to the downloaded tarball on disk.
	Apply(ctx context.Context, src string, log io.Writer) error

	// Backup saves the current live state to a file under dstDir; returns the path.
	Backup(ctx context.Context, dstDir string, log io.Writer) (string, error)

	// Rollback restores from a previously-produced backup file.
	Rollback(ctx context.Context, backup string, log io.Writer) error

	// HealthCheck verifies the component is running correctly after Apply/Rollback.
	HealthCheck(ctx context.Context) error
}

// Registry maps component → Strategy.
type Registry map[shared.Component]Strategy

// New builds the production registry. Job's strategy also requires agent-side
// signed admission and independent configuration.
func New(root string) Registry {
	return Registry{
		shared.CompBackend:  &imageStrategy{name: "jc-backend", root: root, comp: shared.CompBackend},
		shared.CompTGBot:    &unavailableStrategy{comp: shared.CompTGBot},
		shared.CompJob:      &jobStrategy{root: root},
		shared.CompFrontend: &nginxStrategy{name: "jc-frontend", comp: shared.CompFrontend},
		shared.CompAdmin:    &nginxStrategy{name: "jc-admin", comp: shared.CompAdmin},
		shared.CompLang:     &unavailableStrategy{comp: shared.CompLang},
	}
}

// Get returns the strategy for a component or an error.
func (r Registry) Get(c shared.Component) (Strategy, error) {
	s, ok := r[c]
	if !ok {
		return nil, fmt.Errorf("no strategy for component %q", c)
	}
	return s, nil
}

// ErrNoBackup is returned by Rollback when no backup file exists.
var ErrNoBackup = errors.New("no backup available")
