package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

// Asignar con sesión de catálogo soft-borrada → 422 y la tx no deja instancias.
func TestCobertura_UpsertDaySesionCatalogoSoftDeletada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobsoft")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "cobsoft")
	require.NoError(t, daos.NewSessionDao(db).SoftDelete(nil, session.ID))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarSessionNotFound)
	var instanceCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, instanceCount, "el rollback no deja instancias de la operación fallida")
}

// Link de session_exercises cuyo ejercicio ya no existe (borrado en la tabla):
// instantiateSession corta con ErrSessionExerciseNotFound y la tx hace rollback.
func TestCobertura_UpsertDayLinkEjercicioInexistente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "coblink")
	session, exercise := referenciaCatalogSession(t, db, owner.ID, "coblink")
	require.NoError(t, db.Delete(exercise).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
	var exerciseInstanceCount int64
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exerciseInstanceCount).Error)
	assert.Zero(t, exerciseInstanceCount, "el rollback no deja instancias de ejercicios creados antes del error")
}

// validateDayFields, solo branch de horarios: from>=to (y from == to) →
// ErrCalendarInvalidTimeRange; presencial completo válido pasa. El resto de
// las variantes vive en TestCalendarService_Task5_ValidateDayFields_Direct.
func TestCobertura_ValidateDayFieldsRangoHorario(t *testing.T) {
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, nil).(*calendarService)
	isPresencial := true
	loc := &trainingplan.Location{Lat: -34.6, Lng: -58.4}
	from := "10:00"
	to := "09:00"
	tooEarly := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to, PresencialLocation: loc,
	}
	assert.ErrorIs(t, svc.validateDayFields(tooEarly, "", false), ErrCalendarInvalidTimeRange)

	equal := "10:00"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial,
		PresencialTimeFrom: &equal, PresencialTimeTo: &equal, PresencialLocation: loc,
	}
	assert.ErrorIs(t, svc.validateDayFields(req, "", false), ErrCalendarInvalidTimeRange, "from == to también es inválido")

	validTo := "11:00"
	req.PresencialTimeTo = &validTo
	require.NoError(t, svc.validateDayFields(req, "", false))
}

// Banner next_training/next_cancelled con SessionInstanceID huérfana y grupo
// sin nombre: session_name null, group_name vacío, sin error.
func TestCobertura_NextSessionBannerInstanciaHuerfanaYGrupoSinNombre(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobbanner")
	require.NoError(t, db.Model(&dbs.Group{}).Where("id = ?", group.ID).Update("name", "").Error)
	require.NoError(t, db.Model(&dbs.Team{}).Where("id = ?", group.TeamID).Update("name", "").Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	future := time.Now().AddDate(0, 0, 4)
	missing := int64(987654321)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &missing}))
	cancelledDate := future.AddDate(0, 0, 1)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: cancelledDate, Kind: "cancelled", SessionInstanceID: &missing}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp.NextTraining)
	assert.Empty(t, resp.NextTraining.GroupName, "grupo sin nombre responde group_name vacío")
	assert.Nil(t, resp.NextTraining.SessionName, "instancia huérfana deja session_name null sin error")
	assert.False(t, resp.NextTraining.IsPresencial, "día asincrónico no marca presencial")
	assert.Nil(t, resp.NextTraining.PresencialTimeFrom)
	assert.Nil(t, resp.NextTraining.PresencialTimeTo)
	assert.Nil(t, resp.NextTraining.PresencialLocation)
	require.NotNil(t, resp.NextCancelled)
	assert.Equal(t, cancelledDate.Format("2006-01-02"), resp.NextCancelled.Date)
	assert.Empty(t, resp.NextCancelled.GroupName)
	assert.Nil(t, resp.NextCancelled.SessionName)
	// NextSessionBannerItem (base de NextCancelled) no lleva campos presenciales.
}

// Faltante en la doc de referencia: a diferencia del banner, GetRange SÍ rompe
// con error explícito cuando la instancia embebida no existe.
func TestCobertura_GetRangeInstanciaHuerfanaRompe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobgetr")
	future := time.Now().AddDate(0, 0, 4)
	missing := int64(987654321)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.GetRange(nil, group.ID, owner.ID, future, future)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}
