package services

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

// hhmm formatea una hora como "HH:MM" para los requests de calendario.
func hhmm(hour int) string { return fmt.Sprintf("%02d:00", hour) }

func strPtr(s string) *string { return &s }

// presencialRequest arma un CalendarDayRequest training+presencial.
func presencialRequest(fromH, toH int, sessionID *int64) calendar.CalendarDayRequest {
	isPresencial := true
	return calendar.CalendarDayRequest{
		Kind: "training", SessionID: sessionID,
		IsPresencial: &isPresencial, PresencialTimeFrom: strPtr(hhmm(fromH)),
		PresencialTimeTo: strPtr(hhmm(toH)), PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}
}

// presencialPlan arma un plan de 1 día training+presencial para stamp.
func presencialPlan(t *testing.T, db *gorm.DB, ownerID int64, tag string, fromH, toH int, sessionID int64) *dbs.TrainingPlan {
	t.Helper()
	plan := &dbs.TrainingPlan{OwnerID: ownerID, Name: "plan " + tag}
	require.NoError(t, db.Create(plan).Error)
	from := time.Date(0, 1, 1, fromH, 0, 0, 0, time.UTC)
	to := time.Date(0, 1, 1, toH, 0, 0, 0, time.UTC)
	locJSON, err := json.Marshal(trainingplan.Location{Lat: -34.6, Lng: -58.4})
	require.NoError(t, err)
	loc := string(locJSON)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &sessionID,
		DefaultPresencial: true, DefaultTimeFrom: &from, DefaultTimeTo: &to, DefaultLocation: &loc}
	require.NoError(t, db.Create(&pd).Error)
	return plan
}

// presencialWiringSvc arma el service con DAOs reales sobre la DB de test
// (la detección de colisiones consulta por DB, no por mocks).
func presencialWiringSvc(db *gorm.DB, withPlanDaos bool) CalendarServiceInterface {
	var planDao daos.TrainingPlanDaoInterface
	var dayDao daos.PlanDayDaoInterface
	if withPlanDaos {
		planDao = daos.NewTrainingPlanDao(db)
		dayDao = daos.NewPlanDayDao(db)
	}
	return NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, planDao, dayDao, nil, db)
}

// 2.1 — PUT con colisión cross-team → 409 (error tipado) sin escribir el día.
func TestPresencialWiring_UpsertDay_CrossTeamRechazaSinEscribir(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwup1")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwup1-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwup1-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwup1")
	date := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.UpsertDay(nil, groupA.ID, owner.ID, date, presencialRequest(9, 11, &session.ID))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	conflicts := PresencialCollisionConflicts(err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, groupB.ID, conflicts[0].GroupID)
	assert.Equal(t, date.Format("2006-01-02"), conflicts[0].Date)
	assert.Equal(t, "09:00", conflicts[0].PresencialTimeFrom)
	assert.Nil(t, resp)
	written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, date)
	require.NoError(t, err)
	assert.Nil(t, written, "el día colisionante no debe quedar escrito (rollback)")
}

// 2.1 — PUT con superposición same-team → guarda y devuelve same_team_warnings.
func TestPresencialWiring_UpsertDay_SameTeamGuardaConWarnings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwup2")
	groupA2 := extraOwnerTeamGroup(t, db, groupA.TeamID, "pwup2-a2")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwup2")
	date := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupA2.ID, date, 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.UpsertDay(nil, groupA.ID, owner.ID, date, presencialRequest(9, 11, &session.ID))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.SameTeamWarnings, 1)
	assert.Equal(t, groupA2.ID, resp.SameTeamWarnings[0].GroupID)
	assert.Equal(t, groupA.TeamID, resp.SameTeamWarnings[0].TeamID)
	written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, date)
	require.NoError(t, err)
	require.NotNil(t, written, "la superposición same-team no bloquea la escritura")
}

// 2.1 — bordes que se tocan (termina 10:00, arranca 10:00) no son colisión (D1).
func TestPresencialWiring_UpsertDay_BordesQueSeTocanNoColisionan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwup3")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwup3-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwup3-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwup3")
	date := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.UpsertDay(nil, groupA.ID, owner.ID, date, presencialRequest(10, 11, &session.ID))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.SameTeamWarnings)
}

// 2.2 — stamp con colisión cross-team → 409 all-or-nothing y force NO lo bypassa.
func TestPresencialWiring_Stamp_CrossRechazaYForceNoBypassa(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwst1")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwst1-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwst1-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwst1")
	plan := presencialPlan(t, db, owner.ID, "pwst1", 9, 11, session.ID)
	start := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupB.ID, start, 9, 10)
	svc := presencialWiringSvc(db, true)

	resp, err := svc.Stamp(nil, groupA.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: start.Format("2006-01-02")})
	require.Error(t, err, "sin force: la colisión cross rechaza")
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	require.Len(t, PresencialCollisionConflicts(err), 1)
	assert.Equal(t, groupB.ID, PresencialCollisionConflicts(err)[0].GroupID)
	assert.Empty(t, resp.Days, "en error la respuesta es el valor cero")

	respForced, err := svc.Stamp(nil, groupA.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: start.Format("2006-01-02"), Force: true})
	require.Error(t, err, "force NO bypassa la colisión presencial (design.md Non-Goals)")
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	assert.Empty(t, respForced.Days)

	written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, start)
	require.NoError(t, err)
	assert.Nil(t, written, "nada se escribe: all-or-nothing")
}

// 2.2 — stamp con superposición same-team → guarda con warnings en el wrapper.
func TestPresencialWiring_Stamp_SameTeamGuardaConWarnings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwst2")
	groupA2 := extraOwnerTeamGroup(t, db, groupA.TeamID, "pwst2-a2")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwst2")
	plan := presencialPlan(t, db, owner.ID, "pwst2", 9, 11, session.ID)
	start := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupA2.ID, start, 9, 10)
	svc := presencialWiringSvc(db, true)

	resp, err := svc.Stamp(nil, groupA.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: start.Format("2006-01-02")})

	require.NoError(t, err)
	require.Len(t, resp.Days, 1)
	require.Len(t, resp.SameTeamWarnings, 1)
	assert.Equal(t, groupA2.ID, resp.SameTeamWarnings[0].GroupID)
	assert.Equal(t, start.Format("2006-01-02"), resp.Days[0].Date)
}

// 2.3 — bulk con colisión cross en UNA fecha → lote completo rechazado, nada escrito.
func TestPresencialWiring_Bulk_CrossRechazaLoteCompleto(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwbk1")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwbk1-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwbk1-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwbk1")
	d1 := time.Now().AddDate(0, 0, 5)
	d2 := d1.AddDate(0, 0, 1)
	seedPresencialServiceDay(t, db, groupB.ID, d2, 9, 10)
	svc := presencialWiringSvc(db, false)
	req := calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")},
		Kind:  "training", SessionID: &session.ID,
		IsPresencial:       presencialRequest(9, 11, nil).IsPresencial,
		PresencialTimeFrom: strPtr("09:00"), PresencialTimeTo: strPtr("11:00"),
		PresencialLocation: &trainingplan.Location{Lat: 1, Lng: 1},
	}

	resp, err := svc.Bulk(nil, groupA.ID, owner.ID, req)

	require.Error(t, err, "cualquier cross en el lote lo rechaza completo")
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	require.Len(t, PresencialCollisionConflicts(err), 1)
	assert.Equal(t, d2.Format("2006-01-02"), PresencialCollisionConflicts(err)[0].Date)
	assert.Empty(t, resp.Days)
	for _, d := range []time.Time{d1, d2} {
		written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, d)
		require.NoError(t, err)
		assert.Nil(t, written, "all-or-nothing: ninguna fecha del lote se escribe")
	}
}

// 2.3 — bulk same-team → guarda el lote con warnings agregados.
func TestPresencialWiring_Bulk_SameTeamWarningsAgregados(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwbk2")
	groupA2 := extraOwnerTeamGroup(t, db, groupA.TeamID, "pwbk2-a2")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwbk2")
	d1 := time.Now().AddDate(0, 0, 5)
	d2 := d1.AddDate(0, 0, 1)
	seedPresencialServiceDay(t, db, groupA2.ID, d1, 9, 10)
	seedPresencialServiceDay(t, db, groupA2.ID, d2, 9, 10)
	svc := presencialWiringSvc(db, false)
	req := calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")},
		Kind:  "training", SessionID: &session.ID,
		IsPresencial:       presencialRequest(9, 11, nil).IsPresencial,
		PresencialTimeFrom: strPtr("09:00"), PresencialTimeTo: strPtr("11:00"),
		PresencialLocation: &trainingplan.Location{Lat: 1, Lng: 1},
	}

	resp, err := svc.Bulk(nil, groupA.ID, owner.ID, req)

	require.NoError(t, err)
	require.Len(t, resp.Days, 2)
	require.Len(t, resp.SameTeamWarnings, 2, "warnings agregados por cada fecha que superpone")
}

// 2.4 — shift: las filas movidas no colisionan consigo mismas en su fecha nueva.
func TestPresencialWiring_Shift_FilasMovidasNoColisionanConsigoMismas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwsh1")
	d1 := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupA.ID, d1, 9, 10)
	seedPresencialServiceDay(t, db, groupA.ID, d1.AddDate(0, 0, 2), 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.Shift(nil, groupA.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 2})

	require.NoError(t, err, "el destino de una fila movida está ocupado por otra fila movida del mismo shift: no es colisión")
	require.Len(t, resp.Days, 2)
	assert.Empty(t, resp.SameTeamWarnings)
	for _, a := range resp.Days {
		assert.Equal(t, "training", a.Kind)
		assert.True(t, a.IsPresencial)
	}
}

// 2.4 — shift a fecha nueva con colisión cross → 409 y rollback (nada se corre).
func TestPresencialWiring_Shift_CrossEnFechaNuevaRechazaYRollback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwsh2")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwsh2-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwsh2-b")
	d1 := time.Now().AddDate(0, 0, 5)
	d1ID := seedPresencialServiceDay(t, db, groupA.ID, d1, 9, 10)
	seedPresencialServiceDay(t, db, groupB.ID, d1.AddDate(0, 0, 2), 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.Shift(nil, groupA.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 2})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	require.Len(t, PresencialCollisionConflicts(err), 1)
	assert.Equal(t, groupB.ID, PresencialCollisionConflicts(err)[0].GroupID)
	assert.Empty(t, resp.Days)
	unchanged, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, d1)
	require.NoError(t, err)
	require.NotNil(t, unchanged)
	assert.Equal(t, d1ID, unchanged.ID, "rollback: la fila sigue en su fecha original")
}

// 2.4 — shift same-team → guarda con warnings sobre las fechas nuevas.
func TestPresencialWiring_Shift_SameTeamWarningsEnFechasNuevas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwsh3")
	groupA2 := extraOwnerTeamGroup(t, db, groupA.TeamID, "pwsh3-a2")
	d1 := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupA.ID, d1, 9, 10)
	seedPresencialServiceDay(t, db, groupA2.ID, d1.AddDate(0, 0, 2), 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.Shift(nil, groupA.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 2})

	require.NoError(t, err)
	require.Len(t, resp.Days, 1)
	assert.Equal(t, d1.AddDate(0, 0, 2).Format("2006-01-02"), resp.Days[0].Date)
	require.Len(t, resp.SameTeamWarnings, 1)
	assert.Equal(t, groupA2.ID, resp.SameTeamWarnings[0].GroupID)
}
