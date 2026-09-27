package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidPaymentHistoryType(t *testing.T) {
	assert.True(t, IsValidPaymentHistoryType("subscription"))
	assert.True(t, IsValidPaymentHistoryType("trainer_payment"))
	for _, v := range []string{"", "order", "Subscription", "team_subscription"} {
		assert.False(t, IsValidPaymentHistoryType(v), v)
	}
}
