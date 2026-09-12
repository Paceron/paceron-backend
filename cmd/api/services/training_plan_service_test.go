package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type mockTrainingPlanDao struct {
	createFn      func(ctx *gin.Context, p *dbs.TrainingPlan) error
	findByIDFn    func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error)
	updateFn      func(ctx *gin.Context, p *dbs.TrainingPlan) error
	deleteFn      func(ctx *gin.Context, id int64) error
}

func (m *mockTrainingPlanDao) Create(ctx *gin.Context, p *dbs.TrainingPlan) error {
	if m.createFn != nil {
		return m.createFn(ctx, p)
	}
	p.ID = 1
	return nil
}
func (m *mockTrainingPlanDao) FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockTrainingPlanDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockTrainingPlanDao) Update(ctx *gin.Context, p *dbs.TrainingPlan) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}
func (m *mockTrainingPlanDao) Delete(ctx *gin.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

type mockPlanDayDao struct {
	findByPlanFn     func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error)
	replaceForPlanFn func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error
}

func (m *mockPlanDayDao) FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
	if m.findByPlanFn != nil {
		return m.findByPlanFn(ctx, planID)
	}
	return nil, nil
}
func (m *mockPlanDayDao) ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
	if m.replaceForPlanFn != nil {
		return m.replaceForPlanFn(ctx, planID, rows)
	}
	return nil
}

func validPlanDays() []trainingplan.PlanDayRequest {
	return []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación")},
	}
}

func strPtrTP(s string) *string { return &s }

func TestTrainingPlanService_Create_Success(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	resp, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: validPlanDays()})

	require.NoError(t, err)
	assert.Equal(t, "Plan", resp.Name)
}

func TestTrainingPlanService_Create_TooFewDays(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidDayCount)
}

func TestTrainingPlanService_Create_SequenceGap(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 3, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidSequence)
}

func TestTrainingPlanService_Create_TrainingWithoutSessionID(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "training"}, {SequenceNo: 2, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanDayFieldMismatch)
}

func TestTrainingPlanService_Create_TrainingSessionNotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, sessionDao)
	sessionID := int64(5)

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "training", SessionID: &sessionID}, {SequenceNo: 2, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanSessionNotFound)
}

func TestTrainingPlanService_Create_OwnerMismatch(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 99, Name: "Plan", Days: validPlanDays()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestTrainingPlanService_Update_PartialWithoutDays(t *testing.T) {
	existing := &dbs.TrainingPlan{ID: 1, OwnerID: 7, Name: "Viejo"}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) { return existing, nil }}
	dayDaoCalled := false
	dayDao := &mockPlanDayDao{replaceForPlanFn: func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
		dayDaoCalled = true
		return nil
	}}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})
	newName := "Nuevo"

	_, err := svc.Update(nil, 1, 7, trainingplan.TrainingPlanUpdateRequest{Name: &newName})

	require.NoError(t, err)
	assert.False(t, dayDaoCalled, "no debería tocar los días si no vinieron en el body")
}

func TestTrainingPlanService_Delete_Forbidden(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 99}, nil
	}}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}
