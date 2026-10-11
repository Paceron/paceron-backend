package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Task 8: la lectura batch de guard es la PRIMERA select de la tx de stamp y
// bulk; su fallo corta la tx con el mensaje envuelto de cada ruta y sin filas.
func TestDBError_Task8_Stamp_GuardBatchReadFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbtask8s")
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan task8"}
	require.NoError(t, db.Create(plan).Error)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"}
	require.NoError(t, db.Create(&pd).Error)
	// Solo s.db pasa por el pool fallado; la primera select de la tx es el
	// guard batch (FindByGroupAndDates).
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: time.Now().AddDate(0, 0, 300).Format("2006-01-02")})

	require.EqualError(t, err, "error al estampar plan")
	var dayCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ?", group.ID).Count(&dayCount).Error)
	assert.Zero(t, dayCount, "el rollback del guard batch no deja filas")
}

func TestDBError_Task8_Bulk_GuardBatchReadFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbtask8b")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{time.Now().AddDate(0, 0, 300).Format("2006-01-02")}, Kind: "rest"})

	require.EqualError(t, err, "error al aplicar bulk")
	var dayCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ?", group.ID).Count(&dayCount).Error)
	assert.Zero(t, dayCount, "el rollback del guard batch no deja filas")
}
