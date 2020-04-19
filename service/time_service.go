package service

import (
	"context"
	"errors"
	"time"
)

// TimeService is the service that ticks after regular intervals of time.
type TimeService struct {
	// Interval after which the service ticks.
	Interval time.Duration

	ctx  context.Context
	tick chan time.Time
}

// ErrNoRunTimeService is thrown when invalid interval is provided, i.e.,
// the service cannot be run for the given interval.
var ErrNoRunTimeService = errors.New("service cannot be run for given interval")

// NewTimeService creates a time service from the provided `Opts`.
func NewTimeService(ctx context.Context, opts *Opts) *TimeService {
	interval := time.Duration(opts.Interval)
	if opts.Interval <= 0 {
		interval = time.Hour
	}

	return &TimeService{
		Interval: interval,
		ctx:      ctx,
		tick:     make(chan time.Time, 1),
	}
}

// Start begins the execution of time service. It updates the tick stream
// at regular intervals.
func (ts *TimeService) Start() error {
	if ts.Interval <= 0 {
		return ErrNoRunTimeService
	}

	go ts.start()
	return nil
}

func (ts *TimeService) start() {
	// update once when the service starts.
	ts.tick <- time.Now()

	ticker := time.NewTicker(ts.Interval)
	defer ticker.Stop()

	for {
		select {
		case t := <-ticker.C:
			ts.tick <- t

		case <-ts.ctx.Done():
			close(ts.tick)
			return
		}
	}
}

// Tick begins the execution of the time service.
func (ts *TimeService) Tick() <-chan time.Time {
	return ts.tick
}

// Interface guard.
var _ Service = (*TimeService)(nil)
