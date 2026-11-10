package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func seedBatchInstance(t *testing.T, db *gorm.DB, name string, exerciseNames ...string) *dbs.SessionInstance {
	t.Helper()
	session := &dbs.SessionInstance{Name: name}
	require.NoError(t, db.Create(session).Error)
	for _, exerciseName := range exerciseNames {
		exercise := &dbs.ExerciseInstance{Name: exerciseName, Kind: "running"}
		require.NoError(t, db.Create(exercise).Error)
		require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
			SessionInstanceID: session.ID, ExerciseInstanceID: exercise.ID, Role: "main", RepeatCount: 2, RestMinutes: 30,
		}).Error)
	}
	return session
}

// GetRange (batch) y el detalle de fila única comparten mapper: mismo shape y
// mismo orden de ejercicios (por id de link, no por id de ejercicio).
func TestCalendarService_Task1_GetRangeParityWithSingleRowDetail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "task1par")
	session := seedBatchInstance(t, db, "sesion task1", "primero", "segundo")
	date := time.Now().AddDate(0, 0, 3)
	instanceID := session.ID
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &instanceID,
	}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	rangeResp, err := svc.GetRange(nil, group.ID, owner.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rangeResp, 1)
	require.NotNil(t, rangeResp[0].SessionInstance)
	require.Len(t, rangeResp[0].SessionInstance.Exercises, 2)
	assert.Equal(t, "primero", rangeResp[0].SessionInstance.Exercises[0].Name)
	assert.Equal(t, "segundo", rangeResp[0].SessionInstance.Exercises[1].Name)

	detail, err := svc.SessionInstanceDetail(nil, instanceID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, *detail, *rangeResp[0].SessionInstance, "el batch no cambia el shape del detalle")
}

func TestCalendarService_Task1_MemberAndAdministeredCalendarDetailBatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "task1agg")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	base := time.Now().AddDate(0, 0, 10)
	for i := 0; i < 3; i++ {
		session := seedBatchInstance(t, db, fmt.Sprintf("sesion agg %d", i), fmt.Sprintf("ejercicio agg %d", i))
		instanceID := session.ID
		require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{
			GroupID: group.ID, Date: base.AddDate(0, 0, i), Kind: "training", SessionInstanceID: &instanceID,
		}))
	}

	counter := 0
	cdb := countingDB(t, db, &counter)
	countingSvc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	memberResp, err := countingSvc.MemberCalendar(nil, owner.ID, base, base.AddDate(0, 0, 2))
	require.NoError(t, err)
	assert.Equal(t, 5, counter, "memberships + rango + grupos+equipos (JOIN) + instancias + links/ejercicios")

	counter = 0
	adminResp, err := countingSvc.AdministeredCalendar(nil, owner.ID, base, base.AddDate(0, 0, 2))
	require.NoError(t, err)
	assert.Equal(t, 4, counter, "grupos+equipos (JOIN) + rango + instancias + links/ejercicios")

	rangeResp, err := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).
		GetRange(nil, group.ID, owner.ID, base, base.AddDate(0, 0, 2))
	require.NoError(t, err)

	require.Len(t, memberResp, 3)
	require.Len(t, adminResp, 3)
	for i := range rangeResp {
		assert.Equal(t, rangeResp[i], memberResp[i].CalendarDayResponse, "member-calendar conserva el detalle de GetRange")
		assert.Equal(t, rangeResp[i], adminResp[i].CalendarDayResponse, "administered-calendar conserva el detalle de GetRange")
		require.NotNil(t, memberResp[i].SessionInstance)
		assert.Equal(t, rangeResp[i].Date, memberResp[i].Date)
	}
}

func TestCalendarService_Task1_NextSessionBatchNamesAndQueries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "task1next")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	future := time.Now().AddDate(0, 0, 5)
	cancelledSession := seedBatchInstance(t, db, "sesion cancelada task1")
	trainingSession := seedBatchInstance(t, db, "sesion training task1")
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: future, Kind: "cancelled", SessionInstanceID: &cancelledSession.ID,
	}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: future.AddDate(0, 0, 1), Kind: "training", SessionInstanceID: &trainingSession.ID,
	}))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	assert.Equal(t, 4, counter, "memberships + nombres de grupos + banner agrupado + instancias en batch")
	require.NotNil(t, resp.NextCancelled)
	require.NotNil(t, resp.NextTraining)
	assert.Equal(t, cancelledSession.Name, *resp.NextCancelled.SessionName)
	assert.Equal(t, trainingSession.Name, *resp.NextTraining.SessionName)
	assert.Equal(t, future.Format("2006-01-02"), resp.NextCancelled.Date)
	assert.Equal(t, future.AddDate(0, 0, 1).Format("2006-01-02"), resp.NextTraining.Date)
	assert.Equal(t, group.Name, resp.NextTraining.GroupName)
}
