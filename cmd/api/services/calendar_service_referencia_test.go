package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func referenciaCatalogSession(t *testing.T, db *gorm.DB, ownerID int64, tag string) (*dbs.Session, *dbs.Exercise) {
	t.Helper()
	exercise := &dbs.Exercise{OwnerID: ownerID, Name: "ref ex " + tag, Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: ownerID, Name: "ref ses " + tag}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)
	return session, exercise
}

// 4.1 + 4.2 (parte): instantiateSession persista los origenes y la respuesta los expone.
func TestReferencia_UpsertDayPersisteOrigenesYRespondeIDs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refor1")
	session, exercise := referenciaCatalogSession(t, db, owner.ID, "refor1")
	date := time.Now().AddDate(0, 0, 3)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.NoError(t, err)
	require.NotNil(t, resp.SessionInstance)
	require.NotNil(t, resp.SessionInstance.SessionID)
	assert.Equal(t, session.ID, *resp.SessionInstance.SessionID)
	require.Len(t, resp.SessionInstance.Exercises, 1)
	require.NotNil(t, resp.SessionInstance.Exercises[0].ExerciseID)
	assert.Equal(t, exercise.ID, *resp.SessionInstance.Exercises[0].ExerciseID)

	var sessInst dbs.SessionInstance
	require.NoError(t, db.First(&sessInst, resp.SessionInstance.ID).Error)
	require.NotNil(t, sessInst.SourceSessionID)
	assert.Equal(t, session.ID, *sessInst.SourceSessionID)
	var exInsts []dbs.ExerciseInstance
	require.NoError(t, db.Find(&exInsts).Error)
	require.Len(t, exInsts, 1)
	require.NotNil(t, exInsts[0].SourceExerciseID)
	assert.Equal(t, exercise.ID, *exInsts[0].SourceExerciseID)
}

// 4.2: PUT training sin session_id sobre dia con instancia conserva la misma instancia.
func TestReferencia_PutSinSessionIDConservaInstancia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refcon1")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "refcon1")
	date := time.Now().AddDate(0, 0, 3)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	first, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.NoError(t, err)
	require.NotNil(t, first.SessionInstance)
	instanceID := first.SessionInstance.ID

	var sessBefore, exBefore, linkBefore int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&sessBefore).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exBefore).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkBefore).Error)

	second, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training"})
	require.NoError(t, err)
	require.NotNil(t, second.SessionInstance)
	assert.Equal(t, instanceID, second.SessionInstance.ID, "se conserva la misma instancia, no se reinstancia")

	day, err := calendarDao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, err)
	require.NotNil(t, day.SessionInstanceID)
	assert.Equal(t, instanceID, *day.SessionInstanceID)

	var sessAfter, exAfter, linkAfter int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&sessAfter).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exAfter).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkAfter).Error)
	assert.Equal(t, sessBefore, sessAfter, "no se crean ni borran filas de instancia")
	assert.Equal(t, exBefore, exAfter)
	assert.Equal(t, linkBefore, linkAfter)

	require.NotNil(t, second.SessionInstance.SessionID)
	assert.Equal(t, session.ID, *second.SessionInstance.SessionID)
	require.Len(t, second.SessionInstance.Exercises, 1)
	require.NotNil(t, second.SessionInstance.Exercises[0].ExerciseID)
}

// 4.3: PUT training sin session_id sobre dia sin instancia -> 422 FieldMismatch.
func TestReferencia_PutSinSessionIDSinInstanciaRechaza(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refneg1")
	date := time.Now().AddDate(0, 0, 3)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training"})
	assert.ErrorIs(t, err, ErrCalendarFieldMismatch)

	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "rest"})
	require.NoError(t, err)
	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training"})
	assert.ErrorIs(t, err, ErrCalendarFieldMismatch, "dia existente kind=rest tampoco tiene instancia que conservar")
}

// 4.4a: bulk sin session_id sobre N fechas con instancia conserva cada instancia.
func TestReferencia_BulkSinSessionIDConservaPorFecha(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refbk1")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "refbk1")
	d1 := time.Now().AddDate(0, 0, 4)
	d2 := d1.AddDate(0, 0, 1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	first, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")},
		Kind:  "training", SessionID: &session.ID,
	})
	require.NoError(t, err)
	require.Len(t, first.Days, 2)
	idsByDate := map[string]int64{
		first.Days[0].Date: first.Days[0].SessionInstance.ID,
		first.Days[1].Date: first.Days[1].SessionInstance.ID,
	}
	assert.NotEqual(t, idsByDate[first.Days[0].Date], idsByDate[first.Days[1].Date], "cada fecha tenia su propia instancia")

	var sessBefore, exBefore, linkBefore int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&sessBefore).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exBefore).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkBefore).Error)

	second, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")},
		Kind:  "training",
	})
	require.NoError(t, err)
	require.Len(t, second.Days, 2)
	for _, dayResp := range second.Days {
		require.NotNil(t, dayResp.SessionInstance)
		assert.Equal(t, idsByDate[dayResp.Date], dayResp.SessionInstance.ID, "cada fecha conserva su propia instancia")
		require.NotNil(t, dayResp.SessionInstance.SessionID)
		assert.Equal(t, session.ID, *dayResp.SessionInstance.SessionID)
	}

	var sessAfter, exAfter, linkAfter int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&sessAfter).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exAfter).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkAfter).Error)
	assert.Equal(t, sessBefore, sessAfter, "cero filas nuevas o borradas")
	assert.Equal(t, exBefore, exAfter)
	assert.Equal(t, linkBefore, linkAfter)
}

// 4.4b: bulk sin session_id con alguna fecha sin instancia -> 422 con lista de fechas, all-or-nothing.
func TestReferencia_BulkFechaSinInstanciaRechazaLote(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refbk2")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "refbk2")
	d1 := time.Now().AddDate(0, 0, 4)
	d2 := d1.AddDate(0, 0, 1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, d1, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.NoError(t, err)
	day1, err := calendarDao.FindByGroupAndDate(nil, group.ID, d1)
	require.NoError(t, err)
	require.NotNil(t, day1.SessionInstanceID)

	_, err = svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")},
		Kind:  "training",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarTrainingWithoutInstance)
	assert.Contains(t, err.Error(), d2.Format("2006-01-02"), "el error lista la fecha sin instancia")
	assert.NotContains(t, err.Error(), d1.Format("2006-01-02"), "no lista fechas que si tenian instancia")

	day1After, err := calendarDao.FindByGroupAndDate(nil, group.ID, d1)
	require.NoError(t, err)
	require.NotNil(t, day1After.SessionInstanceID)
	assert.Equal(t, *day1.SessionInstanceID, *day1After.SessionInstanceID, "all-or-nothing: la fecha valida no se modifico")
	var day2Count int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", group.ID, d2.Format("2006-01-02")).Count(&day2Count).Error)
	assert.Zero(t, day2Count, "la fecha rechazada no se escribe")
}

// 4.6: instancia con origen NULL (legado de antes del change) responde nulls sin error.
func TestReferencia_InstanciaSinOrigenRespondeNulls(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "refnull1")
	legacyExercise := &dbs.ExerciseInstance{Name: "legado", Kind: "running"}
	require.NoError(t, db.Create(legacyExercise).Error)
	legacySession := &dbs.SessionInstance{Name: "legado ses"}
	require.NoError(t, db.Create(legacySession).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: legacySession.ID, ExerciseInstanceID: legacyExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 2)
	legacyID := legacySession.ID
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &legacyID}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	days, err := svc.GetRange(nil, group.ID, owner.ID, date, date)
	require.NoError(t, err)
	require.Len(t, days, 1)
	require.NotNil(t, days[0].SessionInstance)
	assert.Nil(t, days[0].SessionInstance.SessionID, "instancias legadas responden null")
	require.Len(t, days[0].SessionInstance.Exercises, 1)
	assert.Nil(t, days[0].SessionInstance.Exercises[0].ExerciseID)
}
