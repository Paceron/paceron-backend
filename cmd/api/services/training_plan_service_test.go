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

func TestTrainingPlanService_Create_InvalidDayKind(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "not-a-kind"}, {SequenceNo: 2, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidDayKind)
}

func TestTrainingPlanService_Create_InvalidTimeFormat(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})
	presencial := true
	badTime := "25:99"
	loc := &trainingplan.Location{Lat: 1, Lng: 2}

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación"), DefaultPresencial: &presencial, DefaultTime: &badTime, DefaultLocation: loc},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidTimeFormat)
}

func TestTrainingPlanService_Create_DefaultPresencialSuccess_RoundTripsThroughGet(t *testing.T) {
	var stored []dbs.PlanDay
	var createdPlan *dbs.TrainingPlan
	planDao := &mockTrainingPlanDao{
		createFn: func(ctx *gin.Context, p *dbs.TrainingPlan) error {
			p.ID = 1
			createdPlan = p
			return nil
		},
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return createdPlan, nil
		},
	}
	dayDao := &mockPlanDayDao{
		replaceForPlanFn: func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
			// simulate DB assigning IDs and persisting rows
			for i := range rows {
				rows[i].ID = int64(i + 1)
				rows[i].PlanID = planID
			}
			stored = rows
			return nil
		},
		findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
			return stored, nil
		},
	}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})
	presencial := true
	validTime := "07:30"
	label := "Plaza Central"
	loc := &trainingplan.Location{Lat: -34.6, Lng: -58.4, Label: &label}

	resp, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación"), DefaultPresencial: &presencial, DefaultTime: &validTime, DefaultLocation: loc},
	}})

	require.NoError(t, err)
	require.Len(t, resp.Days, 2)
	presencialDay := resp.Days[1]
	assert.True(t, presencialDay.DefaultPresencial)
	require.NotNil(t, presencialDay.DefaultTime)
	assert.Equal(t, validTime, *presencialDay.DefaultTime)
	require.NotNil(t, presencialDay.DefaultLocation)
	assert.Equal(t, loc.Lat, presencialDay.DefaultLocation.Lat)
	assert.Equal(t, loc.Lng, presencialDay.DefaultLocation.Lng)
	require.NotNil(t, presencialDay.DefaultLocation.Label)
	assert.Equal(t, label, *presencialDay.DefaultLocation.Label)

	// Confirm the same data round-trips through Get too.
	got, err := svc.Get(nil, resp.ID)
	require.NoError(t, err)
	require.Len(t, got.Days, 2)
	assert.True(t, got.Days[1].DefaultPresencial)
	require.NotNil(t, got.Days[1].DefaultTime)
	assert.Equal(t, validTime, *got.Days[1].DefaultTime)
}

func TestTrainingPlanService_Get_NotFound(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) { return nil, nil }}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Get(nil, 1)

	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestTrainingPlanService_Get_Success(t *testing.T) {
	existing := &dbs.TrainingPlan{ID: 1, OwnerID: 7, Name: "Plan"}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) { return existing, nil }}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{ID: 1, PlanID: planID, SequenceNo: 1, Kind: "rest"}}, nil
	}}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})

	resp, err := svc.Get(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, "Plan", resp.Name)
	require.Len(t, resp.Days, 1)
	assert.Equal(t, "rest", resp.Days[0].Kind)
}

func TestTrainingPlanService_List_Success(t *testing.T) {
	plans := []dbs.TrainingPlan{
		{ID: 1, OwnerID: 7, Name: "Plan A"},
		{ID: 2, OwnerID: 7, Name: "Plan B"},
	}
	planDao := &mockTrainingPlanDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) { return plans, nil }}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{ID: planID, PlanID: planID, SequenceNo: 1, Kind: "rest"}}, nil
	}}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, "Plan A", resp[0].Name)
	assert.Equal(t, "Plan B", resp[1].Name)
}

func TestTrainingPlanService_List_Empty(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) { return nil, nil }}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestTrainingPlanService_Clone_Success_DoesNotAffectOriginal(t *testing.T) {
	original := &dbs.TrainingPlan{ID: 1, OwnerID: 7, Name: "Plan Original"}
	originalDays := []dbs.PlanDay{
		{ID: 10, PlanID: 1, SequenceNo: 1, Kind: "rest"},
		{ID: 11, PlanID: 1, SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación")},
	}
	var createdClone *dbs.TrainingPlan
	var clonedRows []dbs.PlanDay
	planDao := &mockTrainingPlanDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			if id == 1 {
				return original, nil
			}
			return createdClone, nil
		},
		createFn: func(ctx *gin.Context, p *dbs.TrainingPlan) error {
			p.ID = 2
			createdClone = p
			return nil
		},
	}
	dayDao := &mockPlanDayDao{
		findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
			if planID == 1 {
				return originalDays, nil
			}
			return clonedRows, nil
		},
		replaceForPlanFn: func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
			for i := range rows {
				rows[i].ID = int64(100 + i)
				rows[i].PlanID = planID
			}
			clonedRows = rows
			return nil
		},
	}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Plan Original (copia)", resp.Name)
	assert.NotEqual(t, original.ID, resp.ID)
	require.Len(t, resp.Days, 2)

	// Original plan/days remain untouched.
	assert.Equal(t, "Plan Original", original.Name)
	assert.Len(t, originalDays, 2)
	assert.Equal(t, int64(10), originalDays[0].ID)
}

func TestTrainingPlanService_Clone_NotFound(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) { return nil, nil }}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestTrainingPlanService_Clone_Forbidden(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 99}, nil
	}}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Clone(nil, 1, 7)

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}
