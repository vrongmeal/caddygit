package caddygit

import (
	"context"
)

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
