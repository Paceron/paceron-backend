package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupOfPaymentStatus(t *testing.T) {
	cases := map[string]PaymentStatusGroup{
		"approved":     PaymentStatusGroupApproved,
		"pending":      PaymentStatusGroupPending,
		"in_process":   PaymentStatusGroupPending,
		"authorized":   PaymentStatusGroupPending,
		"rejected":     PaymentStatusGroupRejected,
		"cancelled":    PaymentStatusGroupRejected,
		"refunded":     PaymentStatusGroupRefunded,
		"charged_back": PaymentStatusGroupRefunded,
		"":             PaymentStatusGroupOther,
		"APPROVED":     PaymentStatusGroupOther,
		"desconocido":  PaymentStatusGroupOther,
	}
	for status, want := range cases {
		assert.Equal(t, want, GroupOfPaymentStatus(status), status)
	}
}

func TestStatusesForGroup_Valid(t *testing.T) {
	statuses, ok := StatusesForGroup("pending")
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"pending", "in_process", "authorized"}, statuses)

	statuses, ok = StatusesForGroup("rejected")
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"rejected", "cancelled"}, statuses)
}

func TestStatusesForGroup_Invalid(t *testing.T) {
	for _, group := range []string{"", "other", "Approved", "failed"} {
		_, ok := StatusesForGroup(group)
		assert.False(t, ok, group)
	}
}
