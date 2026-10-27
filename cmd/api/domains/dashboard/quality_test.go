package dashboard

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataQualityFromSeries(t *testing.T) {
	series := []Series{
		// válida, con tiempo activo, estado reconocido, con runner_session
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("completa"), HasRunnerSession: true},
		// válida, cae al fallback de tiempo, estado reconocido, con runner_session
		{Kind: kindRitmoContinuo, DurationMs: ptrInt64(250000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("partial"), HasRunnerSession: true},
		// válida, sin runner_session: histórico, entra igual al KPI
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("completa")},
		// inválida: ritmo_continuo sin distancia
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), CompletionStatus: ptrString("saltada")},
		// inválida: legacy
		{Kind: kindLegacyWalking, ActiveDurationMs: ptrInt64(1800000), DistanceMeters: ptrFloat(3000), CompletionStatus: ptrString("completa"), HasRunnerSession: true},
	}

	quality := dataQualityFromSeries(series)

	assert.Equal(t, 2, quality.InvalidSeries)
	assert.Equal(t, 1, quality.DegradedTime)
	assert.Equal(t, 1, quality.UnknownStatus)
	assert.Equal(t, 0, quality.OpenSessions, "open_sessions viene de la segunda consulta, no del result set")
	assert.Equal(t, 1, quality.LegacySessions)
	assert.False(t, quality.InsufficientData)
}

func TestDataQualityTodoLimpio(t *testing.T) {
	series := []Series{
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("completa"), HasRunnerSession: true},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(250000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("completa"), HasRunnerSession: true},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(260000), DistanceMeters: ptrFloat(1000), CompletionStatus: ptrString("completa"), HasRunnerSession: true},
	}

	assert.Equal(t, DataQuality{}, dataQualityFromSeries(series))
}

func TestDataQualitySinDatos(t *testing.T) {
	quality := dataQualityFromSeries(nil)

	assert.Equal(t, DataQuality{InsufficientData: true}, quality)
}

// Menos de 3 series válidas con velocidad no trend: el flag lo enciende el
// volumen, no la existencia de filas. Las elongaciones son válidas pero no
// tienen velocidad, así que no cuentan para el umbral.
func TestDataQualityInsufficientData(t *testing.T) {
	series := []Series{
		{Kind: kindElongacion, ActiveDurationMs: ptrInt64(600000), HasRunnerSession: true},
		{Kind: kindElongacion, ActiveDurationMs: ptrInt64(620000), HasRunnerSession: true},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000), HasRunnerSession: true},
	}

	quality := dataQualityFromSeries(series)

	assert.Equal(t, 0, quality.InvalidSeries)
	assert.True(t, quality.InsufficientData, "una sola serie con velocidad no alcanza el umbral")

	ritmo := Series{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(250000), DistanceMeters: ptrFloat(1000), HasRunnerSession: true}

	assert.True(t, dataQualityFromSeries(append(series, ritmo)).InsufficientData, "dos no alcanzan")

	assert.False(t, dataQualityFromSeries(append(series, ritmo, ritmo)).InsufficientData)
}

func TestDataQualityVieneConLasSieteClaves(t *testing.T) {
	raw, err := json.Marshal(DataQuality{})

	assert.NoError(t, err)
	assert.JSONEq(t, `{"invalid_series":0,"degraded_time":0,"unknown_status":0,"open_sessions":0,"legacy_sessions":0,"insufficient_data":false}`, string(raw))
}

// legacy_sessions cubre los dos casos sin respaldo completo: la serie sin fila
// en runner_session y la que entró por el fallback de nombre. Antes eran dos
// cosas distintas y el spec pedía un solo campo para las dos; si alguien
// "optimiza" dejand0 solo una, este test lo frena.
func TestDataQuality_LegacySessionsCountsBothUnbackedPaths(t *testing.T) {
	base := func() Series {
		return Series{Kind: "ritmo_continuo", DurationMs: ptrInt64(240000), DistanceMeters: ptrFloat64x(4000)}
	}

	neither := base()
	neither.HasRunnerSession = true
	neither.ResolvedByName = false

	noRunnerSession := base()
	noRunnerSession.HasRunnerSession = false
	noRunnerSession.ResolvedByName = false

	byName := base()
	byName.HasRunnerSession = true
	byName.ResolvedByName = true

	both := base()
	both.HasRunnerSession = false
	both.ResolvedByName = true

	q := dataQualityFromSeries([]Series{neither, noRunnerSession, byName, both})
	assert.Equal(t, 3, q.LegacySessions, "cuenta las tres series sin respaldo completo")
}

func TestDataQuality_LegacySessionsDoesNotCountInvalidSeries(t *testing.T) {
	// Una serie inválida ya va en invalid_series. Sumarla también a
	// legacy_sessions duplicaría el mismo problema en dos contadores.
	s := Series{Kind: "ritmo_continuo"} // sin tiempos: inválida
	q := dataQualityFromSeries([]Series{s})
	assert.Equal(t, 1, q.InvalidSeries)
	assert.Equal(t, 0, q.LegacySessions)
}

func ptrFloat64x(v float64) *float64 { return &v }
