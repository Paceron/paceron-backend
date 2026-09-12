package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
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
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)

	resp, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Completa", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Completa", resp.Name)
}

func TestSessionService_Create_MissingRole(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Incompleta", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"},
	}})

	assert.ErrorIs(t, err, ErrSessionMissingRole)
}

func TestSessionService_Create_InvalidRole(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "flying"},
	}})

	assert.ErrorIs(t, err, ErrSessionInvalidRole)
}

func TestSessionService_Create_ExerciseNotFound(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
}

func TestSessionService_Create_OwnerMismatch(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 99, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
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
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{})

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
	assert.True(t, replaced)
}
