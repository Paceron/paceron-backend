package mpconnect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			state := BuildState(42, 1699999999999999999, tc.target)

			userID, ts, target, err := ParseState(state)

			assert.NoError(t, err)
			assert.Equal(t, int64(42), userID)
			assert.Equal(t, int64(1699999999999999999), ts)
			assert.Equal(t, tc.want, target)
		})
	}
}

// Los states emitidos antes de que el target existiera tienen 2 segmentos. Se
// siguen aceptando para no romper los que están en vuelo durante el deploy.
func TestParseState_LegacyDosSegmentos(t *testing.T) {
	userID, ts, target, err := ParseState("7-1699999999999999999")

	assert.NoError(t, err)
	assert.Equal(t, int64(7), userID)
	assert.Equal(t, int64(1699999999999999999), ts)
	assert.Equal(t, TargetWeb, target)
}

func TestParseState_Invalido(t *testing.T) {
	cases := []struct {
		name  string
		state string
	}{
		{name: "vacío", state: ""},
		{name: "un solo segmento", state: "42"},
		{name: "userID no numérico", state: "abc-1699999999999999999"},
		{name: "timestamp no numérico", state: "42-abc"},
		{name: "timestamp vacío", state: "42-"},
		{name: "userID vacío", state: "-1699999999999999999"},
		// Un userID negativo parte mal por el separador. No es un caso real
		// (los user_id son PK autoincremental y el controller ya descarta 0),
		// y rechazarlo es preferible a parsearlo a un valor distinto.
		{name: "userID negativo", state: "-1-1699999999999999999-app"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := ParseState(tc.state)

			assert.Error(t, err)
			assert.Equal(t, "formato de state inválido", err.Error())
		})
	}
}

func TestTargetFromState(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  string
	}{
		{name: "target app", state: "42-1699999999999999999-app", want: TargetApp},
		{name: "target web", state: "42-1699999999999999999-web", want: TargetWeb},
		{name: "legacy sin target", state: "42-1699999999999999999", want: TargetWeb},
		{name: "target desconocido", state: "42-1699999999999999999-otro", want: TargetWeb},
		{name: "state corrupto no falla", state: "basura", want: TargetWeb},
		{name: "state vacío no falla", state: "", want: TargetWeb},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TargetFromState(tc.state))
		})
	}
}

