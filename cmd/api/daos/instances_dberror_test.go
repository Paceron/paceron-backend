package daos

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Ramas de error de DB de los DAOs de instancia y de grupo / membresía /
// equipo del área de calendario, inyectadas con FailingDB por tipo de
// operación. Los DAOs envuelven con %w, así que se remarca el mensaje.
func instanceNthFail(op string, n int) func(string) bool {
	calls := map[string]int{}
	return func(o string) bool {
		calls[o]++
		return o == op && calls[o] == n
	}
}

func TestSessionInstanceDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)

	// Create.
	failing := testutils.FailingDB(t, db, instanceNthFail("insert", 1))
	dao := NewSessionInstanceDao(failing)
	err := dao.Create(nil, &dbs.SessionInstance{Name: "ini"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creating session instance")

	// FindByID con select fallida.
	failing = testutils.FailingDB(t, db, instanceNthFail("select", 1))
	dao = NewSessionInstanceDao(failing)
	_, err = dao.FindByID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding session instance")

	// Delete físico.
	failing = testutils.FailingDB(t, db, instanceNthFail("delete", 1))
	dao = NewSessionInstanceDao(failing)
	err = dao.Delete(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting session instance")

	// HasFeedback con select fallida.
	failing = testutils.FailingDB(t, db, instanceNthFail("select", 1))
	dao = NewSessionInstanceDao(failing)
	_, err = dao.HasFeedback(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error checking session instance feedback")
}

func TestSessionInstanceDao_HasFeedbackTrue(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "con feedback real"}
	require.NoError(t, dao.Create(nil, inst))
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"a.jpg"}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		AssignedSessionID: inst.ID, AssignedExerciseID: 1, AthleteUserID: 1,
		FeedbackOwnerUserID: 1, ReportSource: "corredor",
		SessionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), MediaURLs: media,
	}).Error)

	has, err := dao.HasFeedback(nil, inst.ID)

	require.NoError(t, err)
	assert.True(t, has)
}

func TestSessionExerciseInstanceDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)
	sess := &dbs.SessionInstance{Name: "S dbfail"}
	require.NoError(t, sessionDao.Create(nil, sess))
	exInst := &dbs.ExerciseInstance{Name: "E dbfail", Kind: "running", SourceExerciseID: new(int64)}
	require.NoError(t, exerciseDao.Create(nil, exInst))

	// Create.
	failing := testutils.FailingDB(t, db, instanceNthFail("insert", 1))
	dao := NewSessionExerciseInstanceDao(failing)
	err := dao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: exInst.ID, Role: "main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creating session exercise instance")

	// FindByID con select fallida.
	failing = testutils.FailingDB(t, db, instanceNthFail("select", 1))
	dao = NewSessionExerciseInstanceDao(failing)
	_, err = dao.FindByID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding session exercise instance")

	// FindBySessionInstance con select fallida (handle nuevo: el consumido
	// de FindByID ya gastó su match).
	dao = NewSessionExerciseInstanceDao(testutils.FailingDB(t, db, instanceNthFail("select", 1)))
	_, err = dao.FindBySessionInstance(nil, sess.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing session exercise instances")

	// Delete individual.
	failing = testutils.FailingDB(t, db, instanceNthFail("delete", 1))
	dao = NewSessionExerciseInstanceDao(failing)
	err = dao.Delete(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting session exercise instance")

	// DeleteBySessionInstance (borrado en cascada de instancia superada;
	// handle nuevo: el consumido de Delete individual ya gastó el match).
	dao = NewSessionExerciseInstanceDao(testutils.FailingDB(t, db, instanceNthFail("delete", 1)))
	err = dao.DeleteBySessionInstance(nil, sess.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting session exercise instances")
}

func TestExerciseInstanceDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)

	// Create.
	failing := testutils.FailingDB(t, db, instanceNthFail("insert", 1))
	dao := NewExerciseInstanceDao(failing)
	err := dao.Create(nil, &dbs.ExerciseInstance{Name: "E"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creating exercise instance")

	// FindByID con select fallida.
	failing = testutils.FailingDB(t, db, instanceNthFail("select", 1))
	dao = NewExerciseInstanceDao(failing)
	_, err = dao.FindByID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding exercise instance")

	// FindByIDs con select fallida (handle nuevo, igual que FindByID).
	dao = NewExerciseInstanceDao(testutils.FailingDB(t, db, instanceNthFail("select", 1)))
	_, err = dao.FindByIDs(nil, []int64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding exercise instances by ids")

	// FindByIDs sin IDs: vacío sin query.
	dao = NewExerciseInstanceDao(db)
	rows, err := dao.FindByIDs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Delete físico.
	failing = testutils.FailingDB(t, db, instanceNthFail("delete", 1))
	dao = NewExerciseInstanceDao(failing)
	err = dao.Delete(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error deleting exercise instance")

	// HasFeedback con select fallida.
	failing = testutils.FailingDB(t, db, instanceNthFail("select", 1))
	dao = NewExerciseInstanceDao(failing)
	_, err = dao.HasFeedback(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error checking exercise instance feedback")
}

func TestExerciseInstanceDao_HasFeedbackTrue(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	exInst := &dbs.ExerciseInstance{Name: "con feedback real"}
	require.NoError(t, dao.Create(nil, exInst))
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"a.jpg"}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		AssignedSessionID: 1, AssignedExerciseID: exInst.ID, AthleteUserID: 1,
		FeedbackOwnerUserID: 1, ReportSource: "corredor",
		SessionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), MediaURLs: media,
	}).Error)

	has, err := dao.HasFeedback(nil, exInst.ID)

	require.NoError(t, err)
	assert.True(t, has)
}

func TestExerciseInstanceDao_FindByIDs_FlertoInexistentesYPreservaOrden(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	a := &dbs.ExerciseInstance{Name: "A"}
	b := &dbs.ExerciseInstance{Name: "B"}
	require.NoError(t, dao.Create(nil, a))
	require.NoError(t, dao.Create(nil, b))

	rows, err := dao.FindByIDs(nil, []int64{b.ID, a.ID, 999999})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, a.ID, rows[0].ID, "ordenado por id")
	assert.Equal(t, b.ID, rows[1].ID)
}
