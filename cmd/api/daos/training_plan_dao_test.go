package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestTrainingPlanDao_ImplementsInterface(t *testing.T) {
	dao := NewTrainingPlanDao(&gorm.DB{})
	var iface TrainingPlanDaoInterface = dao
	_ = iface
}

func TestTrainingPlanDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainingPlanDao(db)
	owner := persistUser(db, "plan-owner-1@test.com", "63000001")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan base"}

	err := dao.Create(nil, p)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, p.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Plan base", found.Name)
}

func TestTrainingPlanDao_FindByOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainingPlanDao(db)
	owner := persistUser(db, "plan-owner-2@test.com", "63000002")
	require.NoError(t, dao.Create(nil, &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan A"}))
	require.NoError(t, dao.Create(nil, &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan B"}))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestTrainingPlanDao_Delete_CascadesPlanDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dayDao := NewPlanDayDao(db)
	owner := persistUser(db, "plan-owner-3@test.com", "63000003")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "A borrar"}
	require.NoError(t, planDao.Create(nil, p))
	require.NoError(t, dayDao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "rest"},
	}))

	err := planDao.Delete(nil, p.ID)

	require.NoError(t, err)
	found, findErr := planDao.FindByID(nil, p.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
	days, daysErr := dayDao.FindByPlan(nil, p.ID)
	require.NoError(t, daysErr)
	assert.Empty(t, days)
}
