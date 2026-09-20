package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/testutils"
)

type mockExerciseDao struct {
	createFn      func(ctx *gin.Context, e *dbs.Exercise) error
	findByIDFn    func(ctx *gin.Context, id int64) (*dbs.Exercise, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error)
	updateFn      func(ctx *gin.Context, e *dbs.Exercise) error
	softDeleteFn  func(ctx *gin.Context, id int64) error
}

func (m *mockExerciseDao) Create(ctx *gin.Context, e *dbs.Exercise) error {
	if m.createFn != nil {
		return m.createFn(ctx, e)
	}
	e.ID = 1
	return nil
}
func (m *mockExerciseDao) FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockExerciseDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockExerciseDao) Update(ctx *gin.Context, e *dbs.Exercise) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, e)
	}
	return nil
}
func (m *mockExerciseDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestExerciseService_Create_Success(t *testing.T) {
	dao := &mockExerciseDao{}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "Trote", Kind: "jogging"})

	require.NoError(t, err)
	assert.Equal(t, "Trote", resp.Name)
}

func TestExerciseService_Create_InvalidKind(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{}, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "flying"})

	assert.ErrorIs(t, err, ErrExerciseInvalidKind)
}

func TestExerciseService_Create_OwnerMismatch(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{}, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 99, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Update_NotFound(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrExerciseNotFound)
}

func TestExerciseService_Update_Forbidden(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 99}, nil
	}}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Clone_Success(t *testing.T) {
	original := &dbs.Exercise{ID: 1, OwnerID: 7, Name: "Original", Kind: "running"}
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return original, nil }}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
}

func TestExerciseService_Delete_Success(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7}, nil
	}}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
}

func TestExerciseService_Get_Success(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7, Name: "Trote", Kind: "jogging"}, nil
	}}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Get(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, "Trote", resp.Name)
}

func TestExerciseService_Get_NotFound(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Get(nil, 1)

	assert.ErrorIs(t, err, ErrExerciseNotFound)
}

func TestExerciseService_List_Success(t *testing.T) {
	exercises := []dbs.Exercise{{ID: 1, OwnerID: 7, Name: "A", Kind: "running"}, {ID: 2, OwnerID: 7, Name: "B", Kind: "jogging"}}
	dao := &mockExerciseDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) { return exercises, nil }}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].Name)
	assert.Equal(t, "B", resp[1].Name)
}

func TestExerciseService_List_Empty(t *testing.T) {
	dao := &mockExerciseDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) { return nil, nil }}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestExerciseService_Update_NoReferencingDays_SimplePath(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7, Name: "Trote", Kind: "jogging"}, nil
	}}
	calDao := &mockGroupCalendarDao{findByExerciseIDFn: func(ctx *gin.Context, exerciseID int64) ([]dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, calDao, nil)

	resp, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "Trote 50mts", Kind: "jogging"})

	require.NoError(t, err)
	assert.Equal(t, "Trote 50mts", resp.Name)
}

func TestExerciseService_Update_OnlyFutureDay_StaysLiveNoClone(t *testing.T) {
	futureDate := time.Now().AddDate(0, 0, 5)
	repointCalled := false
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7, Name: "Trote", Kind: "jogging"}, nil
	}}
	calDao := &mockGroupCalendarDao{
		findByExerciseIDFn: func(ctx *gin.Context, exerciseID int64) ([]dbs.GroupCalendarDay, error) {
			sessionID := int64(10)
			return []dbs.GroupCalendarDay{{ID: 900, GroupID: 1, SessionInstanceID: &sessionID, Date: futureDate, Kind: "training", IsPresencial: false}}, nil
		},
		repointDaysByIDFn: func(ctx *gin.Context, dayIDs []int64, newSessionID int64) error {
			repointCalled = true
			return nil
		},
	}
	svc := NewExerciseService(dao, &mockSessionDao{}, &mockSessionExerciseDao{}, calDao, nil)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "Trote 50mts", Kind: "jogging"})

	require.NoError(t, err)
	assert.False(t, repointCalled, "un día futuro no debe clonarse ni entrar a la transacción")
}

// TestExerciseService_Update_PastDayLocks_ClonesSessionAndExercise prueba
// contra Postgres real: la rama de congelamiento corre dentro de
// s.db.Transaction con DAOs frescas atadas a la tx (mismo motivo que los
// tests equivalentes de SessionService.Update, D13).
func TestExerciseService_Update_PastDayLocks_ClonesSessionAndExercise(t *testing.T) {
	db := testutils.SetupTestDB(t)
	exerciseDao := daos.NewExerciseDao(db)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewExerciseService(exerciseDao, sessionDao, sessionExerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "exercise-freeze-owner@test.com", DNI: "50000094", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)

	target := &dbs.Exercise{OwnerID: owner.ID, Name: "Correr 100mts", Kind: "running", DistanceM: intPtrEx(100)}
	require.NoError(t, exerciseDao.Create(nil, target))
	warmup := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, warmup))
	cooldown := &dbs.Exercise{OwnerID: owner.ID, Name: "Elongación", Kind: "elongation"}
	require.NoError(t, exerciseDao.Create(nil, cooldown))

	original := &dbs.Session{OwnerID: owner.ID, Name: "Sesión con sprint"}
	require.NoError(t, sessionDao.Create(nil, original))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, original.ID, []dbs.SessionExercise{
		{ExerciseID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: target.ID, Role: "main", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: cooldown.ID, Role: "cooldown", RepeatCount: 1, RestMinutes: 0},
	}))

	team := &dbs.Team{Name: "Equipo freeze ejercicio", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo freeze ejercicio", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	pastDate := time.Now().AddDate(0, 0, -3).Truncate(24 * time.Hour)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: pastDate, Kind: "training", IsPresencial: false, SessionInstanceID: &original.ID}))

	_, err := svc.Update(nil, target.ID, owner.ID, exercise.ExerciseRequest{OwnerID: owner.ID, Name: "Correr 50mts", Kind: "running", DistanceM: intPtrEx(50)})

	require.NoError(t, err)

	// El ejercicio original queda con el valor editado.
	updatedOriginal, err := exerciseDao.FindByID(nil, target.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedOriginal.DistanceM)
	assert.Equal(t, 50, *updatedOriginal.DistanceM)

	// El día pasado quedó repunteado a un clon de la sesión, con un ejercicio
	// clonado que conserva el valor viejo (100mts).
	pastDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, pastDate)
	require.NoError(t, err)
	require.NotNil(t, pastDay.SessionInstanceID)
	assert.NotEqual(t, original.ID, *pastDay.SessionInstanceID, "el día pasado debe repuntear a una sesión clonada")

	clonedExercises, err := sessionExerciseDao.FindBySession(nil, *pastDay.SessionInstanceID)
	require.NoError(t, err)
	require.Len(t, clonedExercises, 3, "el clon debe congelar los 3 ejercicios de la sesión, no solo el editado")

	var clonedMain *dbs.SessionExercise
	for i := range clonedExercises {
		if clonedExercises[i].Role == "main" {
			clonedMain = &clonedExercises[i]
		}
	}
	require.NotNil(t, clonedMain)
	assert.NotEqual(t, target.ID, clonedMain.ExerciseID, "el ejercicio clonado debe ser una fila nueva, no la original")
	clonedExercise, err := exerciseDao.FindByID(nil, clonedMain.ExerciseID)
	require.NoError(t, err)
	require.NotNil(t, clonedExercise.DistanceM)
	assert.Equal(t, 100, *clonedExercise.DistanceM, "el ejercicio clonado debe conservar el valor histórico (100mts)")
}

// TestExerciseService_Update_UsedInTwoSessions_OnlyClosedSessionClones cubre
// D5 (agrupar por sesión): el mismo ejercicio en 2 sesiones con distinto
// estado — solo la sesión con día cerrado se clona.
func TestExerciseService_Update_UsedInTwoSessions_OnlyClosedSessionClones(t *testing.T) {
	db := testutils.SetupTestDB(t)
	exerciseDao := daos.NewExerciseDao(db)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewExerciseService(exerciseDao, sessionDao, sessionExerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "exercise-freeze-multisession@test.com", DNI: "50000095", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	target := &dbs.Exercise{OwnerID: owner.ID, Name: "Sprint", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, target))

	sessionClosed := &dbs.Session{OwnerID: owner.ID, Name: "Sesión con día cerrado"}
	require.NoError(t, sessionDao.Create(nil, sessionClosed))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, sessionClosed.ID, []dbs.SessionExercise{{ExerciseID: target.ID, Role: "main"}}))

	sessionOpen := &dbs.Session{OwnerID: owner.ID, Name: "Sesión solo abierta"}
	require.NoError(t, sessionDao.Create(nil, sessionOpen))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, sessionOpen.ID, []dbs.SessionExercise{{ExerciseID: target.ID, Role: "main"}}))

	team := &dbs.Team{Name: "Equipo multisession", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo multisession", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	pastDate := time.Now().AddDate(0, 0, -2).Truncate(24 * time.Hour)
	futureDate := time.Now().AddDate(0, 0, 10)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: pastDate, Kind: "training", SessionInstanceID: &sessionClosed.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: futureDate, Kind: "training", SessionInstanceID: &sessionOpen.ID}))

	_, err := svc.Update(nil, target.ID, owner.ID, exercise.ExerciseRequest{OwnerID: owner.ID, Name: "Sprint editado", Kind: "running"})

	require.NoError(t, err)

	closedDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, pastDate)
	require.NoError(t, err)
	require.NotNil(t, closedDay.SessionInstanceID)
	assert.NotEqual(t, sessionClosed.ID, *closedDay.SessionInstanceID, "la sesión con día cerrado debe clonarse")

	openDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, futureDate)
	require.NoError(t, err)
	require.NotNil(t, openDay.SessionInstanceID)
	assert.Equal(t, sessionOpen.ID, *openDay.SessionInstanceID, "la sesión sin días cerrados no debe clonarse")
}

// TestExerciseService_Update_SameSessionClosedInOneGroupOpenInAnother cubre
// el escenario 4 de la spec: la MISMA sesión asignada a un día cerrado en un
// grupo y a uno abierto en otro — solo el día cerrado debe repuntearse
// (RepointDaysByID, no RepointSessionForGroups, mismo motivo que D13).
func TestExerciseService_Update_SameSessionClosedInOneGroupOpenInAnother(t *testing.T) {
	db := testutils.SetupTestDB(t)
	exerciseDao := daos.NewExerciseDao(db)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewExerciseService(exerciseDao, sessionDao, sessionExerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "exercise-freeze-mixedgroups@test.com", DNI: "50000096", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	target := &dbs.Exercise{OwnerID: owner.ID, Name: "Fartlek", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, target))

	shared := &dbs.Session{OwnerID: owner.ID, Name: "Sesión compartida"}
	require.NoError(t, sessionDao.Create(nil, shared))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, shared.ID, []dbs.SessionExercise{{ExerciseID: target.ID, Role: "main"}}))

	team := &dbs.Team{Name: "Equipo mixedgroups", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	closedGroup := &dbs.Group{Name: "Grupo cerrado", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(closedGroup).Error)
	openGroup := &dbs.Group{Name: "Grupo abierto", TeamID: team.ID, IsMain: false}
	require.NoError(t, db.Create(openGroup).Error)

	pastDate := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	futureDate := time.Now().AddDate(0, 0, 7)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: closedGroup.ID, Date: pastDate, Kind: "training", SessionInstanceID: &shared.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: openGroup.ID, Date: futureDate, Kind: "training", SessionInstanceID: &shared.ID}))

	_, err := svc.Update(nil, target.ID, owner.ID, exercise.ExerciseRequest{OwnerID: owner.ID, Name: "Fartlek editado", Kind: "running"})

	require.NoError(t, err)

	closedDay, err := calendarDao.FindByGroupAndDate(nil, closedGroup.ID, pastDate)
	require.NoError(t, err)
	require.NotNil(t, closedDay.SessionInstanceID)
	assert.NotEqual(t, shared.ID, *closedDay.SessionInstanceID, "el día cerrado debe repuntear a un clon")

	openDay, err := calendarDao.FindByGroupAndDate(nil, openGroup.ID, futureDate)
	require.NoError(t, err)
	require.NotNil(t, openDay.SessionInstanceID)
	assert.Equal(t, shared.ID, *openDay.SessionInstanceID, "el día abierto de la misma sesión no debe repuntear")
}

func intPtrEx(v int) *int { return &v }
