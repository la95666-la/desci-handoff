package strategies

import (
	"context"
	"errors"
	"io"

	"github.com/joincare/jc-release/internal/shared"
)

var (
	errFairLaunchActivationUnavailable   = errors.New("fair_launch_activation_unavailable")
	errStaticReleaseAdmissionUnavailable = errors.New("static_release_admission_unavailable")
)

// unavailableStrategy is the production implementation for every component until
// the relevant activation and signed artifact-admission authorities exist. It performs no I/O.
type unavailableStrategy struct {
	comp shared.Component
	err  error
}

func (s *unavailableStrategy) Component() shared.Component { return s.comp }
func (s *unavailableStrategy) unavailableError() error {
	if s.err != nil {
		return s.err
	}
	return errFairLaunchActivationUnavailable
}
func (s *unavailableStrategy) Apply(context.Context, string, io.Writer) error {
	return s.unavailableError()
}
func (s *unavailableStrategy) Backup(context.Context, string, io.Writer) (string, error) {
	return "", s.unavailableError()
}
func (s *unavailableStrategy) Rollback(context.Context, string, io.Writer) error {
	return s.unavailableError()
}
func (s *unavailableStrategy) HealthCheck(context.Context) error {
	return s.unavailableError()
}
