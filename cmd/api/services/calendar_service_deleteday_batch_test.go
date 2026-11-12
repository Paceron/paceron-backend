package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func createDayWithInstance(t *testing.T, db *gorm.DB, groupID int64, date time.Time, exerciseNames ...string) (*dbs.GroupCalendarDay, *supersededFixture) {
	t.Helper()
	inst := createSupersededInstance(t, db, "day inst "+date.Format("2006-01-02"), exerciseNames...)
	day := &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", SessionInstanceID: &inst.instance.ID}
	require.NoError(t, db.Create(day).Error)
	return day, inst
}

func deleteDayService(db *gorm.DB) *calendarService {
	return NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, nil, nil, nil, db,
	).(*calendarService)
}

func assertDayDeleted(t *testing.T, db *gorm.DB, groupID int64, date time.Time) {
	t.Helper()
	day, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, groupID, date)
	require.NoError(t, err)
	assert.Nil(t, day, "el día debe borrarse")
}

// Sin feedback: el día, sus links, ejercicios e instancia desaparecen.
func TestDeleteDayBatch_SinFeedbackBorraContenidoEInstancia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "delbatch1")
	date := time.Now().AddDate(0, 0, 3)
	_, inst := createDayWithInstance(t, db, group.ID, date, "e1", "e2", "e3")

	err := deleteDayService(db).DeleteDay(nil, group.ID, owner.ID, date)

	require.NoError(t, err)
	assertDayDeleted(t, db, group.ID, date)
	assertInstanceDeleted(t, db, inst)
}

// D10: feedback activo de la sesión conserva instancia/links/ejercicios; el
// día sí se borra (DeleteDay solo protege la instancia, no el día).
func TestDeleteDayBatch_PreservaPorFeedbackDeSesion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "delbatch2")
	date := time.Now().AddDate(0, 0, 3)
	_, inst := createDayWithInstance(t, db, group.ID, date, "e1", "e2")
	createSupersedeFeedback(t, db, inst.instance.ID, 999999, false)

	err := deleteDayService(db).DeleteDay(nil, group.ID, owner.ID, date)

	require.NoError(t, err)
	assertDayDeleted(t, db, group.ID, date)
	assertInstanceIntact(t, db, inst)
}

// D10: feedback de UN ejercicio protege la instancia entera.
func TestDeleteDayBatch_PreservaPorFeedbackDeEjercicio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "delbatch3")
	date := time.Now().AddDate(0, 0, 3)
	_, inst := createDayWithInstance(t, db, group.ID, date, "e1", "e2", "e3")
	createSupersedeFeedback(t, db, inst.instance.ID, inst.exercises[1].ID, false)

	err := deleteDayService(db).DeleteDay(nil, group.ID, owner.ID, date)

	require.NoError(t, err)
	assertDayDeleted(t, db, group.ID, date)
	assertInstanceIntact(t, db, inst)
}

// Feedback soft-deleted NO protege (misma regla D10 que el batch).
func TestDeleteDayBatch_FeedbackSoftDeletedNoProtege(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "delbatch4")
	date := time.Now().AddDate(0, 0, 3)
	_, inst := createDayWithInstance(t, db, group.ID, date, "e1")
	createSupersedeFeedback(t, db, inst.instance.ID, inst.exercises[0].ID, true)

	err := deleteDayService(db).DeleteDay(nil, group.ID, owner.ID, date)

	require.NoError(t, err)
	assertDayDeleted(t, db, group.ID, date)
	assertInstanceDeleted(t, db, inst)
}

// Día cerrado: mismo 422 de siempre, sin borrar día ni instancia.
func TestDeleteDayBatch_DiaCerradoNoCambiaNada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "delbatch5")
	date := time.Now().AddDate(0, 0, -1)
	day, inst := createDayWithInstance(t, db, group.ID, date, "e1")

	err := deleteDayService(db).DeleteDay(nil, group.ID, owner.ID, date)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	kept, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	require.NotNil(t, kept)
	assert.Equal(t, day.ID, kept.ID)
	assertInstanceIntact(t, db, inst)
}

// Rollback: si falla un delete del lote de supersede, la tx entera vuelve
// atrás — el día, sus links, ejercicios e instancia quedan como estaban.
func TestDeleteDayBatch_RollbackSiBatchDeleteFalla(t *testing.T) {
	// Delete #1 es el día; #2 links batch, #3 ejercicios batch, #4 sesión.
	for _, nth := range []int{2, 3, 4} {
		t.Run(fmt.Sprintf("delete_%d_falla", nth), func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			owner, group := task3OwnerGroup(t, db, fmt.Sprintf("delbatchrb%d", nth))
			date := time.Now().AddDate(0, 0, 3)
			day, inst := createDayWithInstance(t, db, group.ID, date, "e1", "e2")
			failing := testutils.FailingDB(t, db, nthFail("delete", nth))
			svc := deleteDayService(failing)

			err := svc.DeleteDay(nil, group.ID, owner.ID, date)

			require.EqualError(t, err, "error al borrar día de calendario")
			kept, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, date)
			require.NoError(t, findErr)
			require.NotNil(t, kept, "el rollback conserva el día")
			assert.Equal(t, day.ID, kept.ID)
			assertInstanceIntact(t, db, inst)
		})
	}
}

// El batch es fijo: la cantidad de queries de DeleteDay no depende de los
// ejercicios de la instancia (antes era 1+1 feedback + deletes por ejercicio).
func TestDeleteDayBatch_QueriesFijasPorEjercicio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Del", Surname: "Count", Email: "del-count-owner@test.com", DNI: "51009201", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Del count team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Del count group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	date1 := time.Now().AddDate(0, 0, 41)
	date5 := time.Now().AddDate(0, 0, 42)
	_, inst1 := createDayWithInstance(t, db, group.ID, date1, "c1")
	_, inst5 := createDayWithInstance(t, db, group.ID, date5, "c1", "c2", "c3", "c4", "c5")

	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(counting), daos.NewGroupDao(counting), daos.NewTeamDao(counting),
		daos.NewGroupUserDao(counting), nil, nil, nil, nil, counting,
	).(*calendarService)

	require.NoError(t, svc.DeleteDay(nil, group.ID, owner.ID, date1))
	queries1 := counter
	counter = 0
	require.NoError(t, svc.DeleteDay(nil, group.ID, owner.ID, date5))
	queries5 := counter

	t.Logf("DeleteDay: 1 ejercicio=%d queries, 5 ejercicios=%d queries", queries1, queries5)
	assert.Equal(t, queries1, queries5, "el conteo no depende de la cantidad de ejercicios")
	assert.Equal(t, 12, queries1, "2 owner (grupo+equipo) + 1 FindByGroupAndDate pre-tx + 1 FindByGroupAndDate en tx + 1 delete del día + 6 del batch (fb sesión + links + fb ejercicio + 3 deletes IN) + cierre de tx")
	assertInstanceDeleted(t, db, inst1)
	assertInstanceDeleted(t, db, inst5)
}
