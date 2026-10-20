package mpconnect

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testStateSecret = "secret-de-test"

func TestBuildState_RoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{name: "target web", target: TargetWeb, want: TargetWeb},
		{name: "target app", target: TargetApp, want: TargetApp},
		{name: "target desconocido colapsa a web", target: "escritorio", want: TargetWeb},
		{name: "target vacío colapsa a web", target: "", want: TargetWeb},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := BuildState(testStateSecret, 42, 1699999999999999999, tc.target)

			userID, ts, target, err := ParseState(testStateSecret, state)

			require.NoError(t, err)
			assert.Equal(t, int64(42), userID)
			assert.Equal(t, int64(1699999999999999999), ts)
			assert.Equal(t, tc.want, target)
		})
	}
}

func TestParseState_FormatoFirmado(t *testing.T) {
	state := BuildState(testStateSecret, 7, time.Now().UnixNano(), TargetApp)

	assert.Regexp(t, `^7-\d+-app\.[0-9a-f]{64}$`, state)
}

func TestParseState_StateForjadoRechazado(t *testing.T) {
	// Payload bien formado con firma inventada: sin verificación de firma, este
	// state vincularía la cuenta de MP de quien lo manda al usuario 7.
	forjado := "7-1699999999999999999-web.deadbeef"
	_, _, _, err := ParseState(testStateSecret, forjado)

	assert.ErrorIs(t, err, errInvalidState)
}

func TestParseState_TamperingPayloadRechazado(t *testing.T) {
	state := BuildState(testStateSecret, 7, time.Now().UnixNano(), TargetWeb)
	// Misma estructura firmada, otro userID en el payload: la firma ya no
	// corresponde.
	tampered := "8" + state[1:]

	_, _, _, err := ParseState(testStateSecret, tampered)

	assert.ErrorIs(t, err, errInvalidState)
}

func TestParseState_SecretDistintoRechazado(t *testing.T) {
	state := BuildState("secret-a", 7, time.Now().UnixNano(), TargetWeb)

	_, _, _, err := ParseState("secret-b", state)

	assert.ErrorIs(t, err, errInvalidState)
}

// Los states sin firma del formato anterior (2 segmentos) se rechazan: aceptar
// los dejaría abierta la misma vulnerabilidad que firma el state — un state
// legacy es indistinguible de uno forjado.
func TestParseState_LegacySinFirmaRechazado(t *testing.T) {
	_, _, _, err := ParseState(testStateSecret, "7-1699999999999999999")

	assert.ErrorIs(t, err, errInvalidState)
}

func TestParseState_Invalido(t *testing.T) {
	cases := []struct {
		name  string
		state string
	}{
		{name: "vacío", state: ""},
		{name: "sin separador de firma", state: "42"},
		{name: "userID no numérico", state: "abc-1699999999999999999-web.deadbeef"},
		{name: "timestamp no numérico", state: "42-abc-web.deadbeef"},
		{name: "timestamp vacío", state: "42--web.deadbeef"},
		{name: "userID vacío", state: "-1699999999999999999-web.deadbeef"},
		{name: "userID negativo", state: "-1-1699999999999999999-web.deadbeef"},
		{name: "solo firma", state: ".deadbeef"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := ParseState(testStateSecret, tc.state)

			assert.ErrorIs(t, err, errInvalidState)
		})
	}
}

func TestTargetFromState(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  string
	}{
		{name: "firmado target app", state: BuildState(testStateSecret, 42, 1699999999999999999, TargetApp), want: TargetApp},
		{name: "firmado target web", state: BuildState(testStateSecret, 42, 1699999999999999999, TargetWeb), want: TargetWeb},
		{name: "target desconocido", state: "42-1699999999999999999-otro.deadbeef", want: TargetWeb},
		{name: "sin firma", state: "42-1699999999999999999-app", want: TargetApp},
		{name: "state corrupto no falla", state: "basura", want: TargetWeb},
		{name: "state vacío no falla", state: "", want: TargetWeb},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TargetFromState(tc.state))
		})
	}
}
