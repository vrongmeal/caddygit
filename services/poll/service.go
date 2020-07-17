package poll

import (
	"context"
	"errors"
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
}

// CaddyModule returns the Caddy module information.
func (*Service) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.poll",
		New: func() caddy.Module { return new(Service) },
	}
}

// Start begins the execution of poll service. It updates the tick stream
// at regular intervals.
func (s *Service) Start(ctx context.Context) <-chan error {
	tick := make(chan error, 1)

	if s.Interval <= 0 {
		tick <- errors.New("cannot run poll service for non-positive interval")
		close(tick)
		return tick
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
	}(ctx, tick, time.Duration(s.Interval))

	return tick
}

// Interface guard.
var (
	_ caddygit.Service = (*Service)(nil)
	_ caddy.Module     = (*Service)(nil)
)
