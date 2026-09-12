package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/testutils"
)

type mockSessionDao struct {
	createFn      func(ctx *gin.Context, s *dbs.Session) error
	findByIDFn    func(ctx *gin.Context, id int64) (*dbs.Session, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error)
	updateFn      func(ctx *gin.Context, s *dbs.Session) error
	softDeleteFn  func(ctx *gin.Context, id int64) error
}

func (m *mockSessionDao) Create(ctx *gin.Context, s *dbs.Session) error {
	if m.createFn != nil {
		return m.createFn(ctx, s)
	}
	s.ID = 1
	return nil
}
func (m *mockSessionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Session, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockSessionDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockSessionDao) Update(ctx *gin.Context, s *dbs.Session) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, s)
	}
	return nil
}
func (m *mockSessionDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

type mockSessionExerciseDao struct {
	findBySessionFn     func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error)
	replaceForSessionFn func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error
}

func (m *mockSessionExerciseDao) FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
	if m.findBySessionFn != nil {
		return m.findBySessionFn(ctx, sessionID)
	}
	return nil, nil
}
func (m *mockSessionExerciseDao) ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
	if m.replaceForSessionFn != nil {
		return m.replaceForSessionFn(ctx, sessionID, rows)
	}
	return nil
}

func validSessionExercises() []session.SessionExerciseRequest {
	return []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"},
		{ExerciseID: 2, Role: "main"},
		{ExerciseID: 3, Role: "cooldown"},
	}
}

func TestSessionService_Create_Success(t *testing.T) {
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Completa", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Completa", resp.Name)
}

func TestSessionService_Create_MissingRole(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Incompleta", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"},
	}})

	assert.ErrorIs(t, err, ErrSessionMissingRole)
}

func TestSessionService_Create_InvalidRole(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "flying"},
	}})

	assert.ErrorIs(t, err, ErrSessionInvalidRole)
}

func TestSessionService_Create_ExerciseNotFound(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
}

func TestSessionService_Create_OwnerMismatch(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 99, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Update_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Viejo"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Nuevo", resp.Name)
}

func TestSessionService_Update_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Update_Forbidden(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 99}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Delete_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7}
	softDeleted := false
	sessionDao := &mockSessionDao{
		findByIDFn:   func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil },
		softDeleteFn: func(ctx *gin.Context, id int64) error { softDeleted = true; return nil },
	}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, softDeleted)
}

func TestSessionService_Delete_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Get_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Sesión"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Get(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, "Sesión", resp.Name)
}

func TestSessionService_Get_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Get(nil, 1)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_List_Success(t *testing.T) {
	sessions := []dbs.Session{{ID: 1, OwnerID: 7, Name: "A"}, {ID: 2, OwnerID: 7, Name: "B"}}
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return sessions, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].Name)
	assert.Equal(t, "B", resp[1].Name)
}

func TestSessionService_List_Empty(t *testing.T) {
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestSessionService_Clone_DeepCopiesExercises(t *testing.T) {
	original := &dbs.Session{ID: 1, OwnerID: 7, Name: "Original"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return original, nil }}
	replaced := false
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup"}}, nil
		},
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			replaced = true
			return nil
		},
	}
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
	assert.True(t, replaced)
}

// TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints prueba la
// rama de divergencia de Update contra Postgres real: no es mockeable de forma
// significativa sin perder la garantía de atomicidad que el test quiere probar
// (la divergencia corre dentro de s.db.Transaction, con DAOs frescas sobre tx).
func TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-divergence-owner@test.com", DNI: "50000090", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)

	warmup := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote", Kind: "jogging"}
	require.NoError(t, db.Create(warmup).Error)
	main := &dbs.Exercise{OwnerID: owner.ID, Name: "Serie", Kind: "running"}
	require.NoError(t, db.Create(main).Error)
	cooldown := &dbs.Exercise{OwnerID: owner.ID, Name: "Elongación", Kind: "elongation"}
	require.NoError(t, db.Create(cooldown).Error)

	original := &dbs.Session{OwnerID: owner.ID, Name: "Sesión original"}
	require.NoError(t, sessionDao.Create(nil, original))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, original.ID, []dbs.SessionExercise{
		{ExerciseID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: main.ID, Role: "main", RepeatCount: 3, RestMinutes: 2},
		{ExerciseID: cooldown.ID, Role: "cooldown", RepeatCount: 1, RestMinutes: 0},
	}))

	team := &dbs.Team{Name: "Equipo divergencia", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	excludedGroup := &dbs.Group{Name: "Grupo excluido", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(excludedGroup).Error)
	keptGroup := &dbs.Group{Name: "Grupo que se queda", TeamID: team.ID, IsMain: false}
	require.NoError(t, db.Create(keptGroup).Error)

	date := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: excludedGroup.ID, Date: date, Kind: "training", SessionID: &original.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: keptGroup.ID, Date: date, Kind: "training", SessionID: &original.ID}))

	excludeIDs := []int64{excludedGroup.ID}
	newName := "Sesión editada"
	resp, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: newName, ExcludeGroupIDs: &excludeIDs,
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, newName, resp.Name)

	excludedDay, err := calendarDao.FindByGroupAndDate(nil, excludedGroup.ID, date)
	require.NoError(t, err)
	require.NotNil(t, excludedDay.SessionID)
	assert.NotEqual(t, original.ID, *excludedDay.SessionID, "el grupo excluido debe apuntar al clon, no a la sesión original")

	clonedSessionID := *excludedDay.SessionID
	clonedExercises, err := sessionExerciseDao.FindBySession(nil, clonedSessionID)
	require.NoError(t, err)
	assert.Len(t, clonedExercises, 3, "el clon debe tener copia profunda de los 3 ejercicios")

	keptDay, err := calendarDao.FindByGroupAndDate(nil, keptGroup.ID, date)
	require.NoError(t, err)
	require.NotNil(t, keptDay.SessionID)
	assert.Equal(t, original.ID, *keptDay.SessionID, "el grupo no excluido debe seguir apuntando a la sesión original")

	updatedOriginal, err := sessionDao.FindByID(nil, original.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updatedOriginal.Name)
}
