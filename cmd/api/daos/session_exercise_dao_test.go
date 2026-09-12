package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionExerciseDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionExerciseDao(&gorm.DB{})
	var iface SessionExerciseDaoInterface = dao
	_ = iface
}

func TestSessionExerciseDao_ReplaceForSession_FullCycle(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionDao(db)
	exerciseDao := NewExerciseDao(db)
	dao := NewSessionExerciseDao(db)
	owner := persistUser(db, "session-exercise-owner-1@test.com", "62000001")
	s := &dbs.Session{OwnerID: owner.ID, Name: "Con ejercicios"}
	require.NoError(t, sessionDao.Create(nil, s))
	warmup := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote suave", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, warmup))
	main := &dbs.Exercise{OwnerID: owner.ID, Name: "Serie fuerte", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, main))

	err := dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: main.ID, Role: "main", RepeatCount: 3, RestMinutes: 2},
	})

	require.NoError(t, err)
	rows, findErr := dao.FindBySession(nil, s.ID)
	require.NoError(t, findErr)
	require.Len(t, rows, 2)
	assert.Equal(t, "warmup", rows[0].Role)
	assert.Equal(t, "main", rows[1].Role)
}

func TestSessionExerciseDao_ReplaceForSession_ReplacesEntireSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionDao(db)
	exerciseDao := NewExerciseDao(db)
	dao := NewSessionExerciseDao(db)
	owner := persistUser(db, "session-exercise-owner-2@test.com", "62000002")
	s := &dbs.Session{OwnerID: owner.ID, Name: "A reemplazar"}
	require.NoError(t, sessionDao.Create(nil, s))
	first := &dbs.Exercise{OwnerID: owner.ID, Name: "Primero", Kind: "walking"}
	require.NoError(t, exerciseDao.Create(nil, first))
	second := &dbs.Exercise{OwnerID: owner.ID, Name: "Segundo", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, second))
	require.NoError(t, dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: first.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
	}))

	err := dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: second.ID, Role: "cooldown", RepeatCount: 1, RestMinutes: 0},
	})

	require.NoError(t, err)
	rows, findErr := dao.FindBySession(nil, s.ID)
	require.NoError(t, findErr)
	require.Len(t, rows, 1)
	assert.Equal(t, second.ID, rows[0].ExerciseID)
	assert.Equal(t, "cooldown", rows[0].Role)
}
