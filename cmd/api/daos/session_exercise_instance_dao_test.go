package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionExerciseInstanceDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionExerciseInstanceDao(&gorm.DB{})
	var iface SessionExerciseInstanceDaoInterface = dao
	_ = iface
}

func TestSessionExerciseInstanceDao_Create_FindByID_FindBySessionInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)
	sess := &dbs.SessionInstance{Name: "Fartlek 5K"}
	require.NoError(t, sessionDao.Create(nil, sess))
	warmup := &dbs.ExerciseInstance{Name: "Trote", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, warmup))
	main := &dbs.ExerciseInstance{Name: "Serie", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, main))
	link1 := &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0}
	require.NoError(t, linkDao.Create(nil, link1))
	link2 := &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: main.ID, Role: "main", RepeatCount: 3, RestMinutes: 2}
	require.NoError(t, linkDao.Create(nil, link2))

	found, err := linkDao.FindByID(nil, link2.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "main", found.Role)
	assert.Equal(t, main.ID, found.ExerciseInstanceID)

	rows, err := linkDao.FindBySessionInstance(nil, sess.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "warmup", rows[0].Role)
	assert.Equal(t, "main", rows[1].Role)
}

func TestSessionExerciseInstanceDao_FindByID_NotFound_ReturnsNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionExerciseInstanceDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionExerciseInstanceDao_DeleteBySessionInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)
	sessA := &dbs.SessionInstance{Name: "Sesión A"}
	require.NoError(t, sessionDao.Create(nil, sessA))
	sessB := &dbs.SessionInstance{Name: "Sesión B"}
	require.NoError(t, sessionDao.Create(nil, sessB))
	exercise := &dbs.ExerciseInstance{Name: "Trote", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, exercise))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sessA.ID, ExerciseInstanceID: exercise.ID, Role: "warmup"}))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sessA.ID, ExerciseInstanceID: exercise.ID, Role: "main"}))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sessB.ID, ExerciseInstanceID: exercise.ID, Role: "warmup"}))

	require.NoError(t, linkDao.DeleteBySessionInstance(nil, sessA.ID))

	rowsA, err := linkDao.FindBySessionInstance(nil, sessA.ID)
	require.NoError(t, err)
	assert.Empty(t, rowsA)
	rowsB, err := linkDao.FindBySessionInstance(nil, sessB.ID)
	require.NoError(t, err)
	require.Len(t, rowsB, 1)
}

func TestSessionExerciseInstanceDao_Delete_Single(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sess := &dbs.SessionInstance{Name: "Sesión"}
	require.NoError(t, NewSessionInstanceDao(db).Create(nil, sess))
	exercise := &dbs.ExerciseInstance{Name: "Trote", Kind: "jogging"}
	require.NoError(t, NewExerciseInstanceDao(db).Create(nil, exercise))
	link := &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: exercise.ID, Role: "warmup"}
	require.NoError(t, linkDao.Create(nil, link))

	require.NoError(t, linkDao.Delete(nil, link.ID))

	found, err := linkDao.FindByID(nil, link.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionExerciseInstanceDao_FindBySessionInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)
	sessA := &dbs.SessionInstance{Name: "Sesion A"}
	sessB := &dbs.SessionInstance{Name: "Sesion B"}
	require.NoError(t, sessionDao.Create(nil, sessA))
	require.NoError(t, sessionDao.Create(nil, sessB))
	ej := &dbs.ExerciseInstance{Name: "Trote", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, ej))
	linkA := &dbs.SessionExerciseInstance{SessionInstanceID: sessA.ID, ExerciseInstanceID: ej.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0}
	linkB := &dbs.SessionExerciseInstance{SessionInstanceID: sessB.ID, ExerciseInstanceID: ej.ID, Role: "main", RepeatCount: 1, RestMinutes: 1}
	require.NoError(t, linkDao.Create(nil, linkA))
	require.NoError(t, linkDao.Create(nil, linkB))

	rows, err := linkDao.FindBySessionInstances(nil, []int64{sessB.ID, sessA.ID})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, linkA.ID, rows[0].ID)
	assert.Equal(t, linkB.ID, rows[1].ID)

	rows, err = linkDao.FindBySessionInstances(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestSessionExerciseInstanceDao_FindWithExercisesBySessionInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)

	sessA := &dbs.SessionInstance{Name: "Sesion join A"}
	sessB := &dbs.SessionInstance{Name: "Sesion join B"}
	require.NoError(t, sessionDao.Create(nil, sessA))
	require.NoError(t, sessionDao.Create(nil, sessB))
	exA := &dbs.ExerciseInstance{Name: "Ejercicio join A", Kind: "running"}
	exB := &dbs.ExerciseInstance{Name: "Ejercicio join B", Kind: "strength"}
	require.NoError(t, exerciseDao.Create(nil, exA))
	require.NoError(t, exerciseDao.Create(nil, exB))

	// Insertados fuera de orden por sesión: el DAO ordena por instancia y link.
	linkB := &dbs.SessionExerciseInstance{SessionInstanceID: sessB.ID, ExerciseInstanceID: exB.ID, Role: "main", RepeatCount: 2, RestMinutes: 1}
	linkA2 := &dbs.SessionExerciseInstance{SessionInstanceID: sessA.ID, ExerciseInstanceID: exB.ID, Role: "main", RepeatCount: 3}
	linkA1 := &dbs.SessionExerciseInstance{SessionInstanceID: sessA.ID, ExerciseInstanceID: exA.ID, Role: "warmup"}
	require.NoError(t, linkDao.Create(nil, linkA1))
	require.NoError(t, linkDao.Create(nil, linkA2))
	require.NoError(t, linkDao.Create(nil, linkB))

	rows, err := linkDao.FindWithExercisesBySessionInstances(nil, []int64{sessB.ID, sessA.ID})

	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.NotNil(t, rows[0].Exercise)
	require.NotNil(t, rows[1].Exercise)
	require.NotNil(t, rows[2].Exercise)
	assert.Equal(t, []int64{linkA1.ID, linkA2.ID, linkB.ID}, []int64{rows[0].Link.ID, rows[1].Link.ID, rows[2].Link.ID})
	assert.Equal(t, []int64{sessA.ID, sessA.ID, sessB.ID}, []int64{rows[0].Link.SessionInstanceID, rows[1].Link.SessionInstanceID, rows[2].Link.SessionInstanceID})
	assert.Equal(t, "Ejercicio join A", rows[0].Exercise.Name)
	assert.Equal(t, exB.ID, rows[1].Exercise.ID)
	assert.Equal(t, "strength", rows[2].Exercise.Kind)

	rows, err = linkDao.FindWithExercisesBySessionInstances(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestSessionExerciseInstanceDao_FindWithExercisesBySessionInstances_OrphanKeepsLink(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := NewSessionExerciseInstanceDao(db)
	sessionDao := NewSessionInstanceDao(db)

	sess := &dbs.SessionInstance{Name: "Sesion orphan join"}
	require.NoError(t, sessionDao.Create(nil, sess))
	link := &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: 987654321, Role: "main"}
	require.NoError(t, linkDao.Create(nil, link))

	rows, err := linkDao.FindWithExercisesBySessionInstances(nil, []int64{sess.ID})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, link.ID, rows[0].Link.ID)
	assert.Equal(t, int64(987654321), rows[0].Link.ExerciseInstanceID)
	assert.Nil(t, rows[0].Exercise, "el link huérfano se conserva para que el mapper falle explícito")
}
