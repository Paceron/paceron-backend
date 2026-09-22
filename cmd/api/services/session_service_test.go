package services

import (
	"errors"
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

func TestSessionService_Update_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Viejo"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao)

	resp, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Nuevo", resp.Name)
}

func TestSessionService_Update_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Update_Forbidden(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 99}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

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
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, softDeleted)
}

func TestSessionService_Delete_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Get_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Sesión"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	resp, err := svc.Get(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, "Sesión", resp.Name)
}

func TestSessionService_Get_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Get(nil, 1)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_List_Success(t *testing.T) {
	sessions := []dbs.Session{{ID: 1, OwnerID: 7, Name: "A"}, {ID: 2, OwnerID: 7, Name: "B"}}
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return sessions, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].Name)
	assert.Equal(t, "B", resp[1].Name)
}

func TestSessionService_List_Empty(t *testing.T) {
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestSessionService_Clone_CopiesSessionAndExercises(t *testing.T) {
	original := &dbs.Session{ID: 1, OwnerID: 7, Name: "Original"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return original, nil }}
	replaced := false
	var copiedRows []dbs.SessionExercise
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup"}}, nil
		},
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			replaced = true
			copiedRows = rows
			return nil
		},
	}
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{})

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
	assert.True(t, replaced)
	require.Len(t, copiedRows, 1)
	assert.Equal(t, int64(1), copiedRows[0].ExerciseID, "el clon conserva el ExerciseID del original")
}

func strPtrC(s string) *string { return &s }

func TestSessionService_CloneInternal_NilPointerFields(t *testing.T) {
	original := &dbs.Session{ID: 1, OwnerID: 7, Name: "Original", Description: strPtrC("desc")}
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup", RepeatCount: 3, RestMinutes: 2}}, nil
		},
	}
	name := "Pedaleo fuerte"
	description := "columna alta"

	clone, err := cloneSessionInternal(sessionDao, sessionExerciseDao, nil, original, &name, &description)

	require.NoError(t, err)
	assert.Equal(t, "Pedaleo fuerte", clone.Name)
	assert.Equal(t, "columna alta", *clone.Description)
}

func TestSessionService_CloneInternal_FindExercisesError(t *testing.T) {
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return nil, errors.New("db down")
		},
	}

	_, err := cloneSessionInternal(sessionDao, sessionExerciseDao, nil, &dbs.Session{ID: 1, OwnerID: 7, Name: "X"}, nil, nil)

	assert.Error(t, err)
}

func TestSessionService_CloneInternal_CreateError(t *testing.T) {
	sessionDao := &mockSessionDao{createFn: func(ctx *gin.Context, s *dbs.Session) error { return errors.New("db down") }}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup"}}, nil
		},
	}

	_, err := cloneSessionInternal(sessionDao, sessionExerciseDao, nil, &dbs.Session{ID: 1, OwnerID: 7, Name: "X"}, nil, nil)

	assert.Error(t, err)
}

func TestSessionService_CloneInternal_ReplaceError(t *testing.T) {
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup"}}, nil
		},
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			return errors.New("db down")
		},
	}

	_, err := cloneSessionInternal(sessionDao, sessionExerciseDao, nil, &dbs.Session{ID: 1, OwnerID: 7, Name: "X"}, nil, nil)

	assert.Error(t, err)
}

func TestSessionService_ValidateExercises_FindError(t *testing.T) {
	err := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, errors.New("db down") },
	}).(*sessionService).validateExercises(nil, validSessionExercises())

	assert.Error(t, err)
}

func TestSessionService_ToResponse_FindError(t *testing.T) {
	_, err := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return nil, errors.New("db down")
		},
	}, &mockExerciseDao{}).(*sessionService).toResponse(nil, &dbs.Session{ID: 1, OwnerID: 7, Name: "X"})
	assert.Error(t, err)
}

func TestSessionService_ToSessionExerciseRows_Overrides(t *testing.T) {
	repeat := 5
	rest := 10
	rows := toSessionExerciseRows([]session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "main", RepeatCount: &repeat, RestMinutes: &rest},
		{ExerciseID: 2, Role: "cooldown"},
	})

	assert.Equal(t, 5, rows[0].RepeatCount)
	assert.Equal(t, 10, rows[0].RestMinutes)
	assert.Equal(t, 1, rows[1].RepeatCount)
	assert.Equal(t, 0, rows[1].RestMinutes)
}

func TestSessionService_Create_CreateError(t *testing.T) {
	sessionDao := &mockSessionDao{createFn: func(ctx *gin.Context, s *dbs.Session) error { return errors.New("db down") }}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.Error(t, err)
}

func TestSessionService_Create_ReplaceError(t *testing.T) {
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			return errors.New("db down")
		},
	}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.Error(t, err)
}

func TestSessionService_Update_FindError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return nil, errors.New("db down")
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.Error(t, err)
}

func TestSessionService_Update_InvalidRole(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 7}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "flying"},
	}})

	assert.ErrorIs(t, err, ErrSessionInvalidRole)
}

func TestSessionService_Update_UpdateError(t *testing.T) {
	sessionDao := &mockSessionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
			return &dbs.Session{ID: id, OwnerID: 7}, nil
		},
		updateFn: func(ctx *gin.Context, s *dbs.Session) error { return errors.New("db down") },
	}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.Error(t, err)
}

func TestSessionService_Update_ReplaceError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 7}, nil
	}}
	sessionExerciseDao := &mockSessionExerciseDao{
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			return errors.New("db down")
		},
	}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.Error(t, err)
}

func TestSessionService_Delete_FindError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return nil, errors.New("db down")
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	err := svc.Delete(nil, 1, 7)

	assert.Error(t, err)
}

func TestSessionService_Delete_Forbidden(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 99}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Delete_SoftDeleteError(t *testing.T) {
	sessionDao := &mockSessionDao{
		findByIDFn:   func(ctx *gin.Context, id int64) (*dbs.Session, error) { return &dbs.Session{ID: id, OwnerID: 7}, nil },
		softDeleteFn: func(ctx *gin.Context, id int64) error { return errors.New("db down") },
	}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	err := svc.Delete(nil, 1, 7)

	assert.Error(t, err)
}

func TestSessionService_Clone_FindError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return nil, errors.New("db down")
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.Error(t, err)
}

func TestSessionService_Clone_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Clone_Forbidden(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 99}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Clone_InternalError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 7, Name: "X"}, nil
	}}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return nil, errors.New("db down")
		},
	}
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.Error(t, err)
}

func TestSessionService_Get_FindError(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return nil, errors.New("db down")
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Get(nil, 1)

	assert.Error(t, err)
}

func TestSessionService_List_FindError(t *testing.T) {
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
		return nil, errors.New("db down")
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.List(nil, 7)

	assert.Error(t, err)
}

func TestSessionService_List_ToResponseError(t *testing.T) {
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
		return []dbs.Session{{ID: 1, OwnerID: 7, Name: "A"}}, nil
	}}
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return nil, errors.New("db down")
		},
	}
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{})

	_, err := svc.List(nil, 7)

	assert.Error(t, err)
}
