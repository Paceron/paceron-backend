package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/exercise"
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
	svc := NewExerciseService(dao)

	resp, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "Trote", Kind: "jogging"})

	require.NoError(t, err)
	assert.Equal(t, "Trote", resp.Name)
}

func TestExerciseService_Create_InvalidKind(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{})

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "flying"})

	assert.ErrorIs(t, err, ErrExerciseInvalidKind)
}

func TestExerciseService_Create_OwnerMismatch(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{})

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 99, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Update_NotFound(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewExerciseService(dao)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrExerciseNotFound)
}

func TestExerciseService_Update_Forbidden(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 99}, nil
	}}
	svc := NewExerciseService(dao)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Clone_Success(t *testing.T) {
	original := &dbs.Exercise{ID: 1, OwnerID: 7, Name: "Original", Kind: "running"}
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return original, nil }}
	svc := NewExerciseService(dao)

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
}

func TestExerciseService_Delete_Success(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7}, nil
	}}
	svc := NewExerciseService(dao)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
}
