package httpclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_AbremPorUmbralYSeRecupera(t *testing.T) {
	breaker := NewCircuitBreaker(2, 10*time.Millisecond)

	require.NoError(t, breaker.Allow())
	breaker.Fail()
	require.NoError(t, breaker.Allow(), "con un fallo todavía está cerrado")
	breaker.Fail()

	require.ErrorIs(t, breaker.Allow(), ErrCircuitBreakerOpen)

	time.Sleep(15 * time.Millisecond)
	require.NoError(t, breaker.Allow(), "pasado el timeout pasa a half-open")

	breaker.Success()
	require.NoError(t, breaker.Allow())
	assert.Equal(t, StateClosed, breaker.state)
	assert.Zero(t, breaker.failures)
}

func TestCircuitBreaker_SuccessCierraAunqueHayaFallos(t *testing.T) {
	breaker := NewCircuitBreaker(3, time.Hour)
	breaker.Fail()
	breaker.Fail()
	breaker.Success()

	assert.Equal(t, StateClosed, breaker.state)
	assert.Zero(t, breaker.failures)
	require.NoError(t, breaker.Allow())
}

func TestCircuitBreaker_FailSinLlegarAlUmbralNoAbre(t *testing.T) {
	breaker := NewCircuitBreaker(3, time.Hour)
	breaker.Fail()
	breaker.Fail()

	assert.Equal(t, StateClosed, breaker.state)
	require.NoError(t, breaker.Allow())
}
