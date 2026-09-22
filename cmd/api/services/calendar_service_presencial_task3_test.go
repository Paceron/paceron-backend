package services

import (
	"encoding/json"
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

// asyncTrainingRequest arma un request training SIN presencial (entrenamiento
// async): no debe disparar la detección de colisiones.
func asyncTrainingRequest(sessionID int64) calendar.CalendarDayRequest {
	return calendar.CalendarDayRequest{Kind: "training", SessionID: &sessionID}
}

// presencialPlan2Days arma un plan con 2 días secuenciales training+presencial
// (mismo rango horario) para pruebas de exclude_dates en stamp.
func presencialPlan2Days(t *testing.T, db *gorm.DB, ownerID int64, tag string, sessionID int64, fromH, toH int) *dbs.TrainingPlan {
	plan := presencialPlan(t, db, ownerID, tag, fromH, toH, sessionID)
	from := time.Date(0, 1, 1, fromH, 0, 0, 0, time.UTC)
	to := time.Date(0, 1, 1, toH, 0, 0, 0, time.UTC)
	locJSON, err := json.Marshal(trainingplan.Location{Lat: -34.6, Lng: -58.4})
	require.NoError(t, err)
	loc := string(locJSON)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 2, Kind: "training", SessionID: &sessionID,
		DefaultPresencial: true, DefaultTimeFrom: &from, DefaultTimeTo: &to, DefaultLocation: &loc}
	require.NoError(t, db.Create(&pd).Error)
	return plan
}

// 3.4 — un día presencial cancelado superpuesto NO bloquea la escritura: la
//
//	query de días presenciales activos ya lo excluye.
func TestPresencialWiring_UpsertDay_CanceladoSuperpuestoNoBloquea(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwcx1")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwcx1-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwcx1-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwcx1")
	date := time.Now().AddDate(0, 0, 5)
	cancelledFrom, cancelledTo := presencialServiceTimes(9, 10)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: groupB.ID, Date: date, Kind: "cancelled", IsPresencial: true,
		PresencialTimeFrom: cancelledFrom, PresencialTimeTo: cancelledTo,
	}))
	svc := presencialWiringSvc(db, false)

	resp, err := svc.UpsertDay(nil, groupA.ID, owner.ID, date, presencialRequest(9, 10, &session.ID))

	require.NoError(t, err, "el día cancelado del otro equipo no debe bloquear")
	require.NotNil(t, resp)
	assert.Empty(t, resp.SameTeamWarnings)
}

// 3.8 — regresión: escrituras no presenciales no pasan por la detección.
//
//	Un training async superpuesto con un presencial de otro equipo NO colisiona.
func TestPresencialWiring_UpsertDay_AsyncNoDetectaColisiones(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwcx2")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwcx2-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwcx2-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwcx2")
	date := time.Now().AddDate(0, 0, 5)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)
	svc := presencialWiringSvc(db, false)

	resp, err := svc.UpsertDay(nil, groupA.ID, owner.ID, date, asyncTrainingRequest(session.ID))

	require.NoError(t, err, "un día no presencial nunca dispara la detección")
	require.NotNil(t, resp)
	assert.Empty(t, resp.SameTeamWarnings)
	written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, date)
	require.NoError(t, err)
	require.NotNil(t, written)
	assert.False(t, written.IsPresencial)
}

// 3.5 — stamp con colisión cross en una fecha NO excluida → 409
//
//	all-or-nothing, aunque la otra fecha del plan esté excluida.
func TestPresencialWiring_Stamp_CrossEnFechaNoExcluidaRechaza(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwcx3")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwcx3-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwcx3-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwcx3")
	start := time.Now().AddDate(0, 0, 5)
	d2 := start.AddDate(0, 0, 1)
	plan := presencialPlan2Days(t, db, owner.ID, "pwcx3", session.ID, 9, 11)
	seedPresencialServiceDay(t, db, groupB.ID, d2, 9, 10) // colisión en la 2ª fecha (no excluida)
	svc := presencialWiringSvc(db, true)
	req := calendar.StampRequest{
		PlanID: plan.ID, StartDate: start.Format("2006-01-02"),
		ExcludeDates: []string{start.Format("2006-01-02")},
	}

	resp, err := svc.Stamp(nil, groupA.ID, owner.ID, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarPresencialCollision)
	conflicts := PresencialCollisionConflicts(err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, groupB.ID, conflicts[0].GroupID)
	assert.Equal(t, d2.Format("2006-01-02"), conflicts[0].Date)
	assert.Empty(t, resp.Days, "all-or-nothing: la respuesta es el valor cero en error")
	// Ninguna fecha del lote se escribe, ni siquiera la no excluida:
	// la colisión aborta la transacción completa.
	for _, d := range []time.Time{start, d2} {
		written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, d)
		require.NoError(t, err)
		assert.Nil(t, written, "nada se escribe cuando un cross rechaza el lote")
	}
}

// 3.5 — colisión SOLO en una fecha excluida → stamp sin errores: la fecha
//
//	excluida no se estampa y no dispara detección; el resto del plan sigue.
func TestPresencialWiring_Stamp_ColisionSoloEnFechaExcluidaGuarda(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwcx4")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwcx4-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwcx4-b")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "pwcx4")
	start := time.Now().AddDate(0, 0, 5)
	d2 := start.AddDate(0, 0, 1)
	plan := presencialPlan2Days(t, db, owner.ID, "pwcx4", session.ID, 9, 11)
	seedPresencialServiceDay(t, db, groupB.ID, start, 9, 10) // colisión solo en la 1ª fecha
	svc := presencialWiringSvc(db, true)
	req := calendar.StampRequest{
		PlanID: plan.ID, StartDate: start.Format("2006-01-02"),
		ExcludeDates: []string{start.Format("2006-01-02")},
	}

	resp, err := svc.Stamp(nil, groupA.ID, owner.ID, req)

	require.NoError(t, err, "la fecha excluida no se estampa y no dispara detección")
	require.Len(t, resp.Days, 1, "solo la fecha no excluida aparece en la respuesta")
	assert.Equal(t, d2.Format("2006-01-02"), resp.Days[0].Date)
	assert.Empty(t, resp.SameTeamWarnings)
	// La fecha excluida de groupA no se creó; la del grupo colisionante queda intacta.
	excluded, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, start)
	require.NoError(t, err)
	assert.Nil(t, excluded, "la fecha excluida no se toca")
	kept, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupB.ID, start)
	require.NoError(t, err)
	require.NotNil(t, kept)
	assert.Equal(t, groupB.ID, kept.GroupID)
	// La fecha no excluida sí quedó persistida con los datos del plan.
	written, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupA.ID, d2)
	require.NoError(t, err)
	require.NotNil(t, written, "la fecha no excluida se estampa")
	assert.Equal(t, "training", written.Kind)
	assert.True(t, written.IsPresencial)
	require.NotNil(t, written.PresencialTimeFrom)
	require.NotNil(t, written.PresencialTimeTo)
	assert.Equal(t, 9, written.PresencialTimeFrom.UTC().Hour())
	assert.Equal(t, 11, written.PresencialTimeTo.UTC().Hour())
	require.NotNil(t, written.SourcePlanID)
	assert.Equal(t, plan.ID, *written.SourcePlanID)
	require.NotNil(t, written.SessionInstanceID, "training del plan instancia su sesión")
	var sessInst dbs.SessionInstance
	require.NoError(t, db.First(&sessInst, *written.SessionInstanceID).Error)
	require.NotNil(t, sessInst.SourceSessionID)
	assert.Equal(t, session.ID, *sessInst.SourceSessionID)
}

// Pendiente diferido de Task 1 — excludeGroupID: un día colisionante
//
//	perteneciente al grupo excluido no se reporta.
func TestFindPresencialCollisions_ExcludeGroupID_OmiteElGrupoExcluido(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "pwcx5")
	teamB := extraOwnerTeam(t, db, owner.ID, "pwcx5-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "pwcx5-b")
	date := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).(*calendarService)
	candidates := []dbs.GroupCalendarDay{presencialCandidate(groupA.ID, date, 9, 10)}

	cross, same, err := svc.findPresencialCollisions(nil, db, owner.ID, &groupB.ID, nil, []time.Time{date}, candidates)

	require.NoError(t, err)
	assert.Empty(t, cross, "el grupo excluido no debe reportarse como colisionante")
	assert.Empty(t, same)

	cross, same, err = svc.findPresencialCollisions(nil, db, owner.ID, nil, nil, []time.Time{date}, candidates)

	require.NoError(t, err)
	require.Len(t, cross, 1, "sin exclusión, la misma fila sí colisiona")
	assert.Equal(t, groupB.ID, cross[0].GroupID)
	assert.Empty(t, same)
}
