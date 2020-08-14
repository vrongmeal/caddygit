package poll

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caddyserver/caddy/v2"

	"github.com/vrongmeal/caddygit"
)

func init() {
	caddy.RegisterModule(&Service{})
}

// Service is the service that ticks after regular intervals of time.
type Service struct {
	// Interval after which the service ticks.
	Interval caddy.Duration `json:"interval,omitempty"`

	tick chan error
}

// CaddyModule returns the Caddy module information.
func (*Service) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.poll",
		New: func() caddy.Module { return new(Service) },
	}
}

// Provision set's s's configuration for the module.
func (s *Service) Provision(ctx caddy.Context) error {
	if s.Interval <= 0 {
		s.Interval = caddy.Duration(time.Hour)
	}

	s.tick = make(chan error, 1)
	return nil
}

// Validate validates s's configuration.
func (s *Service) Validate() error {
	if s.Interval < caddy.Duration(5*time.Second) {
		return fmt.Errorf(
			"minimum poll time should be 5 seconds; given %s",
			time.Duration(s.Interval),
		)
	}

	return nil
}

// ConfigureRepo configures "s" with the repository information.
func (s *Service) ConfigureRepo(r caddygit.RepositoryInfo) error { return nil }

// Start begins the execution of poll service. It updates the tick stream
// at regular intervals.
func (s *Service) Start(ctx context.Context) <-chan error {
	if s.Interval <= 0 {
		s.tick <- errors.New("cannot run poll service for non-positive interval")
		close(s.tick)
		return s.tick
	}

	go func(c context.Context, t chan<- error, interval time.Duration) {
		// update once when the service starts.
		t <- nil

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t <- nil

			case <-ctx.Done():
				t <- ctx.Err()
				close(t)
				return
			}
		}
	}(ctx, s.tick, time.Duration(s.Interval))

	return s.tick
}

// Interface guard.
var (
	_ caddygit.Service  = (*Service)(nil)
	_ caddy.Module      = (*Service)(nil)
	_ caddy.Provisioner = (*Service)(nil)
	_ caddy.Validator   = (*Service)(nil)
)
