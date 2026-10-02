package instance

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// El DTO de instancia es compartido por los paths de calendario (D9): los 3
// campos del estado presencial (Gap 26 D8) solo aparecen cuando el detalle
// los setea — nil ⇒ ausentes del JSON.
func TestSessionInstanceResponse_PresencialFieldsOmittedByDefault(t *testing.T) {
	sess := dbs.SessionInstance{ID: 1, Name: "Fartlek"}
	resp, err := NewSessionResponse(sess, nil, nil)
	require.NoError(t, err)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(raw), "presencial_open"))
	assert.False(t, strings.Contains(string(raw), "opened_at"))
	assert.False(t, strings.Contains(string(raw), "closed_at"))
}

func TestApplyPresencialState(t *testing.T) {
	opened := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)

	t.Run("día presencial abierta", func(t *testing.T) {
		resp := SessionInstanceResponse{}
		ApplyPresencialState(&resp, &dbs.GroupCalendarDay{IsPresencial: true, PresencialOpenedAt: &opened})
		require.NotNil(t, resp.PresencialOpen)
		assert.True(t, *resp.PresencialOpen)
		require.NotNil(t, resp.PresencialOpenedAt)
		assert.Nil(t, resp.PresencialClosedAt)
	})
	t.Run("día presencial cerrada", func(t *testing.T) {
		resp := SessionInstanceResponse{}
		ApplyPresencialState(&resp, &dbs.GroupCalendarDay{IsPresencial: true, PresencialOpenedAt: &opened, PresencialClosedAt: &closed})
		require.NotNil(t, resp.PresencialOpen)
		assert.False(t, *resp.PresencialOpen)
		require.NotNil(t, resp.PresencialClosedAt)
	})
	t.Run("día no presencial", func(t *testing.T) {
		resp := SessionInstanceResponse{}
		ApplyPresencialState(&resp, &dbs.GroupCalendarDay{})
		assert.Nil(t, resp.PresencialOpen)
		assert.Nil(t, resp.PresencialOpenedAt)
		assert.Nil(t, resp.PresencialClosedAt)
	})
	t.Run("día inexistente", func(t *testing.T) {
		resp := SessionInstanceResponse{}
		ApplyPresencialState(&resp, nil)
		assert.Nil(t, resp.PresencialOpen)
		assert.Nil(t, resp.PresencialOpenedAt)
		assert.Nil(t, resp.PresencialClosedAt)
	})
}
