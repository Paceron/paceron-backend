package httpclient

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitBreakerOpen = errors.New(
	"circuit breaker is open",
)

type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half-open"
)

type CircuitBreaker struct {
	mutex sync.Mutex

	state CircuitBreakerState

	failures         int
	failureThreshold int

	resetTimeout time.Duration
	lastFailure  time.Time
}

func NewCircuitBreaker(
	failureThreshold int,
	resetTimeout time.Duration,
) *CircuitBreaker {

	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

func (cb *CircuitBreaker) Allow() error {

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {

	case StateOpen:

		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = StateHalfOpen
			return nil
		}

		return ErrCircuitBreakerOpen

	default:
		return nil
	}
}

func (cb *CircuitBreaker) Success() {

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures = 0
	cb.state = StateClosed
}

func (cb *CircuitBreaker) Fail() {

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.failureThreshold {
		cb.state = StateOpen
	}
}
