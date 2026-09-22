package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

func TestCalendarService_Task3_ClosedUpsertDoesNotWrite(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	yesterday := time.Now().AddDate(0, 0, -1)
	wrote := false
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "rest"}, nil
		},
		upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
			wrote = true
			return nil
		},
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, yesterday, calendar.CalendarDayRequest{Kind: "rest"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cerrado")
	assert.False(t, wrote)
}

func TestCalendarService_Task3_CancelClosedDayIsAllowedAndKeepsInstance(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	past := time.Now().AddDate(0, 0, -1)
	instanceID := int64(44)
	var saved *dbs.GroupCalendarDay
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return &dbs.GroupCalendarDay{GroupID: groupID, Date: past, Kind: "training", SessionInstanceID: &instanceID}, nil
		},
		upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error { saved = day; return nil },
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	reason := "clima"

	_, err := svc.UpsertDay(nil, 1, 7, past, calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, instanceID, *saved.SessionInstanceID)
}

func TestCalendarService_Task3_TrainingAssignmentCreatesIndependentInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Task3", Surname: "Owner", Email: "task3-instance-owner@test.com", DNI: "50990001", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "Task3 exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "Task3 session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main", RepeatCount: 2, RestMinutes: 3}).Error)

	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil,
		nil, nil, daos.NewSessionDao(db), db,
	)
	date := time.Now().AddDate(0, 0, 2)
	sessionID := session.ID

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &sessionID})

	require.NoError(t, err)
	var day dbs.GroupCalendarDay
	require.NoError(t, db.Where("group_id = ? AND date = ?", group.ID, date).First(&day).Error)
	require.NotNil(t, day.SessionInstanceID)
	assert.NotEqual(t, session.ID, *day.SessionInstanceID)
	var instance dbs.SessionInstance
	require.NoError(t, db.First(&instance, *day.SessionInstanceID).Error)
	assert.Equal(t, session.Name, instance.Name)
	var links []dbs.SessionExerciseInstance
	require.NoError(t, db.Where("session_instance_id = ?", instance.ID).Find(&links).Error)
	assert.Len(t, links, 1)
	var instances []dbs.ExerciseInstance
	require.NoError(t, db.Find(&instances, links[0].ExerciseInstanceID).Error)
	assert.Len(t, instances, 1)
	assert.Equal(t, exercise.Name, instances[0].Name)
}

func TestCalendarService_Task3_BulkClosedDateRollsBackWholeBatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Task3", Surname: "Bulk", Email: "task3-bulk-owner@test.com", DNI: "50990002", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 bulk team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 bulk group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	past := time.Now().AddDate(0, 0, -1)
	future := time.Now().AddDate(0, 0, 2)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "rest"}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{past.Format("2006-01-02"), future.Format("2006-01-02")}, Kind: "rest"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCalendarDayClosed))
	day, findErr := calendarDao.FindByGroupAndDate(nil, group.ID, future)
	require.NoError(t, findErr)
	assert.Nil(t, day)
}

func TestCalendarService_Task3_ReassignmentKeepsInstanceWithFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Task3", Surname: "Feedback", Email: "task3-feedback-owner@test.com", DNI: "50990003", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 feedback team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 feedback group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	oldExercise := &dbs.ExerciseInstance{Name: "old", Kind: "running"}
	require.NoError(t, db.Create(oldExercise).Error)
	oldSession := &dbs.SessionInstance{Name: "old session"}
	require.NoError(t, db.Create(oldSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: oldSession.ID, ExerciseInstanceID: oldExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 2)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	oldID := oldSession.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &oldID}))
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/task3.jpg"}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		AssignedSessionID: oldSession.ID, AssignedExerciseID: oldExercise.ID, AthleteUserID: owner.ID,
		FeedbackOwnerUserID: owner.ID, ReportSource: "atleta", SessionDate: date, MediaURLs: media,
	}).Error)
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "new", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "new session"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &catalogSession.ID})

	require.NoError(t, err)
	var kept dbs.SessionInstance
	require.NoError(t, db.First(&kept, oldSession.ID).Error)
	assert.Equal(t, oldSession.Name, kept.Name)
}

func TestCalendarService_Task3_GetRangeEmbedsFrozenInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Task3", Surname: "Response", Email: "task3-response-owner@test.com", DNI: "50990004", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 response team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 response group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "frozen exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "frozen session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 2)
	require.NoError(t, func() error {
		_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
		return err
	}())

	responses, err := svc.GetRange(nil, group.ID, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.NotNil(t, responses[0].SessionInstance)
	assert.Equal(t, session.Name, responses[0].SessionInstance.Name)
	require.Len(t, responses[0].SessionInstance.Exercises, 1)
	assert.Equal(t, exercise.Name, responses[0].SessionInstance.Exercises[0].Name)
}

func TestCalendarService_Task3_GetRangePropagatesMissingExerciseFromMapper(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Task3", Surname: "Mapper", Email: "task3-mapper-owner@test.com", DNI: "50990005", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 mapper team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 mapper group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	session := &dbs.SessionInstance{Name: "broken instance"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: session.ID, ExerciseInstanceID: 999999, Role: "main"}).Error)
	id := session.ID
	date := time.Now().AddDate(0, 0, 2)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &id}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.GetRange(nil, group.ID, owner.ID, date, date)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "faltante")
}

func TestCalendarService_Task3_ClosedDeleteAndBulkClearAreRejected(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	past := time.Now().AddDate(0, 0, -1)
	deleted := false
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return &dbs.GroupCalendarDay{GroupID: groupID, Date: past, Kind: "rest"}, nil
		},
		deleteFn:        func(ctx *gin.Context, groupID int64, date time.Time) error { deleted = true; return nil },
		deleteByDatesFn: func(ctx *gin.Context, groupID int64, dates []time.Time) error { deleted = true; return nil },
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 7, past)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCalendarDayClosed))
	err = svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{past.Format("2006-01-02")}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCalendarDayClosed))
	assert.False(t, deleted)
}

func TestCalendarService_Task3_ClosedShiftIsRejectedBeforeTransaction(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	past := time.Now().AddDate(0, 0, -1)
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: groupID, Date: past, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: past.Format("2006-01-02"), Days: 1})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCalendarDayClosed))
}

func TestCalendarService_Task3_StampValidationErrorsStayTyped(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{SequenceNo: 1, Kind: "training"}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarFieldMismatch)
}

func TestCalendarService_Task3_BulkInvalidTimeStaysTyped(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	isPresencial := true
	from, to := "bad", "19:00"
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{
		Dates: []string{"2999-10-01"}, Kind: "rest", IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarInvalidTimeFormat)
}

func task3OwnerGroup(t *testing.T, db *gorm.DB, tag string) (*dbs.User, *dbs.Group) {
	t.Helper()
	owner := &dbs.User{Name: "Task3", Surname: tag, Email: "task3-" + tag + "@test.com", DNI: "50990" + tag}
	owner.BirthDate = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	owner.Password = "hashed"
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Task3 " + tag + " team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Task3 " + tag + " group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return owner, group
}

func TestCalendarService_Task3_ReassignmentWithoutFeedbackDeletesOldInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "deleteold")
	oldExercise := &dbs.ExerciseInstance{Name: "old", Kind: "running"}
	require.NoError(t, db.Create(oldExercise).Error)
	oldSession := &dbs.SessionInstance{Name: "old session"}
	require.NoError(t, db.Create(oldSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: oldSession.ID, ExerciseInstanceID: oldExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 2)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	oldID := oldSession.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &oldID}))
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "new", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "new session"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &catalogSession.ID})

	require.NoError(t, err)
	var sessionCount, exerciseCount, linkCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Where("id = ?", oldSession.ID).Count(&sessionCount).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Where("id = ?", oldExercise.ID).Count(&exerciseCount).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Where("session_instance_id = ?", oldSession.ID).Count(&linkCount).Error)
	assert.Zero(t, sessionCount)
	assert.Zero(t, exerciseCount)
	assert.Zero(t, linkCount)
}

func TestCalendarService_Task3_StampClosedDateRollsBackFutureDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "stampclosed")
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "stamp exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "stamp session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "stamp closed plan"}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, db.Create([]dbs.PlanDay{{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"}, {PlanID: plan.ID, SequenceNo: 2, Kind: "training", SessionID: &session.ID}}).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, db)
	start := time.Now().AddDate(0, 0, -1)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: start.Format("2006-01-02")})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	var dayCount, instanceCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ?", group.ID).Count(&dayCount).Error)
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, dayCount)
	assert.Zero(t, instanceCount)
}

func TestCalendarService_Task3_StampForceReplacesFutureInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "stampforce")
	oldExercise := &dbs.ExerciseInstance{Name: "old force", Kind: "running"}
	require.NoError(t, db.Create(oldExercise).Error)
	oldSession := &dbs.SessionInstance{Name: "old force session"}
	require.NoError(t, db.Create(oldSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: oldSession.ID, ExerciseInstanceID: oldExercise.ID, Role: "main"}).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "force exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "force session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "force plan"}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, db.Create(&dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &session.ID}).Error)
	date := time.Now().AddDate(0, 0, 2)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	oldID := oldSession.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &oldID}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, db)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: date.Format("2006-01-02"), Force: true})

	require.NoError(t, err)
	day, findErr := calendarDao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	require.NotNil(t, day.SessionInstanceID)
	assert.NotEqual(t, oldSession.ID, *day.SessionInstanceID)
	var oldCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Where("id = ?", oldSession.ID).Count(&oldCount).Error)
	assert.Zero(t, oldCount)
}

func TestCalendarService_Task3_BulkClearClosedDayRollsBackWithRealDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "bulkclear")
	calendarDao := daos.NewGroupCalendarDayDao(db)
	past := time.Now().AddDate(0, 0, -1)
	future := time.Now().AddDate(0, 0, 2)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "rest"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "rest"}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	err := svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{past.Format("2006-01-02"), future.Format("2006-01-02")}})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	remaining, findErr := calendarDao.FindByGroupAndRange(nil, group.ID, past, future)
	require.NoError(t, findErr)
	assert.Len(t, remaining, 2)
}

func TestCalendarService_Task3_CurrentDayClosureVariants(t *testing.T) {
	// `now` fijo al mediodía local: con time.Now() real este test era flaky entre
	// 00:00 y 01:00 (now.Add(-1h) cruzaba a la víspera y el umbral HH:MM invertía
	// el resultado esperado).
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today}, now), "async current day must be closed")
	futureTime := time.Date(0, 1, 1, 13, 0, 0, 0, time.UTC)
	pastTime := time.Date(0, 1, 1, 11, 0, 0, 0, time.UTC)
	assert.False(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today, IsPresencial: true, PresencialTimeFrom: &futureTime}, now))
	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today, IsPresencial: true, PresencialTimeFrom: &pastTime}, now))
}

func TestCalendarService_Task3_ValidationAndConflictErrorsRemainTyped(t *testing.T) {
	validationErrors := []error{
		ErrCalendarInvalidKind, ErrCalendarFieldMismatch, ErrCalendarInvalidCancelTransition,
		ErrCalendarInvalidTimeFormat, ErrCalendarInvalidTimeRange, ErrCalendarSessionNotFound,
		ErrSessionExerciseNotFound,
	}
	for _, validationErr := range validationErrors {
		assert.True(t, isCalendarValidationError(validationErr), validationErr.Error())
	}
	assert.False(t, isCalendarValidationError(errors.New("database failure")))

	conflict := &calendarStampConflictError{dates: []string{"2026-10-01", "2026-10-03"}}
	assert.Contains(t, conflict.Error(), "2026-10-01")
	assert.ErrorIs(t, conflict, ErrCalendarStampConflict)
}

func TestCalendarService_Task3_PlanDayRequestMapsPresencialDefaults(t *testing.T) {
	from := time.Date(0, 1, 1, 18, 30, 0, 0, time.UTC)
	to := time.Date(0, 1, 1, 19, 30, 0, 0, time.UTC)
	location := `{"lat":-34.6,"lng":-58.4,"label":"pista"}`

	req, err := calendarRequestFromPlanDay(dbs.PlanDay{Kind: "rest", DefaultPresencial: true, DefaultTimeFrom: &from, DefaultTimeTo: &to, DefaultLocation: &location})

	require.NoError(t, err)
	require.NotNil(t, req.IsPresencial)
	assert.True(t, *req.IsPresencial)
	assert.Equal(t, "18:30", *req.PresencialTimeFrom)
	assert.Equal(t, "19:30", *req.PresencialTimeTo)
	require.NotNil(t, req.PresencialLocation)
	assert.Equal(t, -34.6, req.PresencialLocation.Lat)
}
