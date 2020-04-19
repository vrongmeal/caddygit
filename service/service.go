// Package service implements the time interval and webhook services
// for the Caddy Git module.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/caddyserver/caddy/v2"
)

// Service is anything that runs throughout the runtime of the program
// and updates on event. The service should be context based, i.e.,
// stops when the context is canceled.
//
// A service can be used with a for range loop as such:
//
//  s.Start()
//  for t := range s.Tick() {
//  	fmt.Println(t) // prints time when service ticks
//  }
//
type Service interface {
	// Start begins the execution of service asynchronously.
	Start() error

	// Tick receives the time when the service needs to update.
	Tick() <-chan time.Time
}

// Type is the type of service supported by the caddygit service.
type Type int

// Types of services.
const (
	TimeServiceType Type = iota
	WebhookServiceType
)

// UnmarshalJSON satisfies json.Unmarshaler according to
// this type's documentation.
func (t *Type) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case ``, `""`, `0`, `"0"`, `"time"`, `"time_service"`:
		*t = TimeServiceType

	case `1`, `"1"`, `"webhook"`, `"webhook_service"`:
		*t = WebhookServiceType

	default:
		return errors.New("invalid service type: " + string(b))
	}

	return nil
}

// Opts are options that can be used to create a new service.
type Opts struct {
	// Type of the service. Can be "time" or "webhook". Defaults to "time".
	// WIP: webhook service.
	Type Type `json:"type,omitempty"`

	// TimeService options:-

	// Interval after which service will tick. Defaults to 1 hour.
	Interval caddy.Duration `json:"interval,omitempty"`
}

// ErrInvalidService is thrown when an undefined service type is passed
// through opts.
var ErrInvalidService = errors.New("invalid service type")

// New creates an instance of service from `Opts`.
func New(ctx context.Context, opts *Opts) (Service, error) {
	switch opts.Type {
	case TimeServiceType:
		return NewTimeService(ctx, opts), nil
	case WebhookServiceType:
	default:
		return nil, ErrInvalidService
	}

	return nil, nil
}
