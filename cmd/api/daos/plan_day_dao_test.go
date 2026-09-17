package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestPlanDayDao_ImplementsInterface(t *testing.T) {
	dao := NewPlanDayDao(&gorm.DB{})
	var iface PlanDayDaoInterface = dao
	_ = iface
}

func TestPlanDayDao_ReplaceForPlan_OrderedBySequence(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dao := NewPlanDayDao(db)
	owner := persistUser(db, "planday-owner-1@test.com", "64000001")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Con días"}
	require.NoError(t, planDao.Create(nil, p))

	err := dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 2, Kind: "rest"},
		{SequenceNo: 1, Kind: "other", OtherName: strPtr("Descanso activo")},
	})

	require.NoError(t, err)
	days, findErr := dao.FindByPlan(nil, p.ID)
	require.NoError(t, findErr)
	require.Len(t, days, 2)
	assert.Equal(t, 1, days[0].SequenceNo)
	assert.Equal(t, 2, days[1].SequenceNo)
}

func TestPlanDayDao_ReplaceForPlan_ReplacesEntireSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dao := NewPlanDayDao(db)
	owner := persistUser(db, "planday-owner-2@test.com", "64000002")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "A reemplazar"}
	require.NoError(t, planDao.Create(nil, p))
	require.NoError(t, dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 2, Kind: "rest"}, {SequenceNo: 3, Kind: "rest"},
	}))

	err := dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"},
	})

	require.NoError(t, err)
	days, findErr := dao.FindByPlan(nil, p.ID)
	require.NoError(t, findErr)
	assert.Len(t, days, 1)
}

func strPtr(s string) *string { return &s }
