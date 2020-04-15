package caddygit

import (
	"sync"
	"time"
)

// service runs a specific runner function at regular intervals.
type service struct {
	interval time.Duration

	runnerFunc func() error
	errorFunc  func(error)

	kill chan bool
	wg   *sync.WaitGroup
}

// newService returns a new instance of service.
func newService(
	runnerFunc func() error,
	errorFunc func(error),
	interval time.Duration) *service {
	return &service{
		interval: interval,

		runnerFunc: runnerFunc,
		errorFunc:  errorFunc,

		kill: make(chan bool, 1),
		wg:   &sync.WaitGroup{},
	}
}

// start starts the execution of the service.
func (s *service) start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(s.interval)

		for {
			select {
			case <-ticker.C:
				if err := s.runnerFunc(); err != nil {
					s.errorFunc(err)
				}

			case <-s.kill:
				return
			}
		}
	}()

	s.wg.Wait()
}

// quit stops the execution of service.
func (s *service) quit() {
	s.kill <- true
	s.wg.Wait()
	close(s.kill)
}
