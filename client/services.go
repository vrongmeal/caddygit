package client

import (
	"context"
	"errors"
	"time"

	"github.com/caddyserver/caddy/v2"
)

func init() {
	caddy.RegisterModule(PollService{})
}

// Service is anything that runs throughout the runtime of the program
// and updates on event. The service should be context based, i.e.,
// stops when the context is canceled.
//
// A service can be used with a for range loop as such:
//
//  for err := range s.Start(ctx) {
//  	if err != nil {
//			// handle error
//  	}
//  	// do something
//  }
//
type Service interface {
	// Start receives the time when the service needs to update.
	Start(context.Context) <-chan error
}

// PollService is the service that ticks after regular intervals of time.
type PollService struct {
	// Interval after which the service ticks.
	Interval caddy.Duration `json:"interval,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (ps PollService) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.poll",
		New: func() caddy.Module { return new(PollService) },
	}
}

// Start begins the execution of poll service. It updates the tick stream
// at regular intervals.
func (ps *PollService) Start(ctx context.Context) <-chan error {
	tick := make(chan error, 1)

	if ps.Interval <= 0 {
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
	}(ctx, tick, time.Duration(ps.Interval))

	return tick
}

// Interface guard.
var (
	_ Service      = (*PollService)(nil)
	_ caddy.Module = (*PollService)(nil)
)
