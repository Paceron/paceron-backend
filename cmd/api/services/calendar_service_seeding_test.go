package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// setSeedingFlag prende/apaga SEEDING_ENABLED como lo haría la config del
// servidor. El flag se lee adentro de isCalendarDayClosed justamente para no
// cambiar su firma (calendar_service_task3_test.go:424 y
// calendar_service_task5_test.go:321 la llaman con dos argumentos).
func setSeedingFlag(t *testing.T, enabled bool) {
	t.Helper()
	previous := config.Seeding
	config.Seeding = enabled
	t.Cleanup(func() { config.Seeding = previous })
}

func seedingTestNow() (today time.Time, past time.Time) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today, today.AddDate(0, 0, -3)
}

func TestSeeding_SinBanderaElPasadoSigueCerrado(t *testing.T) {
	setSeedingFlag(t, false)
	today, past := seedingTestNow()

	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: past}, today.Add(12*time.Hour)))
}

// El seed de histórico no tiene otra vía que la API: con la bandera activa la
// única rama que se relaja es `date < hoy` (design.md D14).
func TestSeeding_ConBanderaElPasadoSeAbrie(t *testing.T) {
	setSeedingFlag(t, true)
	today, past := seedingTestNow()

	assert.False(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: past}, today.Add(12*time.Hour)))
}

// El flag no abre el día en curso: la presencial de hoy ya empezada y el async
// de hoy siguen cerrando, porque cargar histórico y estampar el día que se está
// corriendo siguen siendo reglas distintas.
func TestSeeding_ConBanderaElDiaDeHoySigueCerrando(t *testing.T) {
	setSeedingFlag(t, true)
	today, _ := seedingTestNow()
	now := today.Add(12 * time.Hour)
	started := time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC)
	notStarted := time.Date(0, 1, 1, 15, 0, 0, 0, time.UTC)

	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today, IsPresencial: true, PresencialTimeFrom: &started}, now),
		"la presencial de hoy ya empezada sigue cerrada")
	assert.False(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today, IsPresencial: true, PresencialTimeFrom: &notStarted}, now),
		"la presencial de hoy que todavía no empieza sigue abierta")
	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today}, now), "el async de hoy sigue cerrado")
}

// `seeding_enabled` en el body se ignora: la decisión es del servidor, no del
// cliente. El DTO no tiene el campo, así que encoding/json lo descarta.
func TestSeeding_BodyConSeedingEnabledSeIgnora(t *testing.T) {
	setSeedingFlag(t, false)
	_, past := seedingTestNow()

	var stamp calendar.StampRequest
	require.NoError(t, json.Unmarshal([]byte(`{"plan_id":1,"start_date":"2026-01-05","seeding_enabled":true}`), &stamp))
	assert.False(t, config.SeedingEnabled(), "el body no puede prender la bandera")

	var day calendar.CalendarDayRequest
	require.NoError(t, json.Unmarshal([]byte(`{"kind":"other","seeding_enabled":true}`), &day))

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: past}, now),
		"con la bandera apagada el pasado sigue cerrado aunque el body la pida")
}

func TestSeeding_StampFechaPasadaConBandera(t *testing.T) {
	db := testutils.SetupTestDB(t)
	setSeedingFlag(t, true)
	owner, group := task3OwnerGroup(t, db, "seed1")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "seed1")
	plan := presencialPlan(t, db, owner.ID, "seed1", 9, 10, session.ID)
	svc := presencialWiringSvc(db, true)

	startDate := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startDate})

	require.NoError(t, err, "con la bandera activa el stamp de histórico tiene que andar")
	require.Len(t, resp.Days, 1)
	assert.Equal(t, startDate, resp.Days[0].Date)
}

func TestSeeding_StampFechaPasadaSinBanderaFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	setSeedingFlag(t, false)
	owner, group := task3OwnerGroup(t, db, "seed2")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "seed2")
	plan := presencialPlan(t, db, owner.ID, "seed2", 9, 10, session.ID)
	svc := presencialWiringSvc(db, true)

	startDate := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startDate})

	// El controller mapea ErrCalendarDayClosed a 422.
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
}

func TestSeeding_PresencialDeHoyYaEmpezadaSigueRechazadaConBandera(t *testing.T) {
	db := testutils.SetupTestDB(t)
	setSeedingFlag(t, true)
	owner, group := task3OwnerGroup(t, db, "seed3")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "seed3")
	svc := presencialWiringSvc(db, false)

	today := time.Now()
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, today, presencialRequest(0, 1, &session.ID))

	require.Error(t, err, "el flag de seeding no abre la presencial que ya arrancó hoy")
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
}
