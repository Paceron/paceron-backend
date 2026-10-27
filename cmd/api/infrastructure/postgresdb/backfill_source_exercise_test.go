package postgresdb_test

import (
	"testing"

	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedBackfill arma el caso mínimo: un owner, su catálogo y tres instancias —
// una con link, dos huérfanas (una con tilde, otra sinOwnercatalogueable).
func seedBackfill(t *testing.T, db *gorm.DB) (ownerID, linkedID, accentID, orphanID int64) {
	t.Helper()

	require.NoError(t, db.Exec(`INSERT INTO users (id, name, surname, email, password, dni, birth_date, status, created_at, updated_at)
		VALUES (9001, 'Owner', 'Backfill', 'owner@backfill.test', 'x', '39000001', '1990-01-01', 'active', now(), now())`).Error)
	ownerID = 9001

	require.NoError(t, db.Exec(`INSERT INTO teams (id, owner_id, name, max_members, created_at, updated_at)
		VALUES (9001, ?, 'Equipo Backfill', 30, now(), now())`, ownerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO groups (id, team_id, name, created_at, updated_at)
		VALUES (9001, 9001, 'Grupo Backfill', now(), now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO group_users (id, group_id, user_id, date_start, created_at, updated_at)
		VALUES (9001, 9001, 9001, '2020-01-01', now(), now())`).Error)

	require.NoError(t, db.Exec(`INSERT INTO sessions (id, owner_id, name, created_at, updated_at)
		VALUES (9001, ?, 'Sesion Backfill', now(), now())`, ownerID).Error)

	// Catálogo: dos nombres que al normalizar NO colisionan.
	require.NoError(t, db.Exec(`INSERT INTO exercises (id, owner_id, name, kind, minutes, distance_m, created_at, updated_at)
		VALUES (9001, ?, 'Distancia 4 km', 'ritmo_continuo', 30, 4000, now(), now())`, ownerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO exercises (id, owner_id, name, kind, minutes, created_at, updated_at)
		VALUES (9002, ?, 'Trote', 'liviano', 10, now(), now())`, ownerID).Error)

	require.NoError(t, db.Exec(`INSERT INTO session_instances (id, name, source_session_id, created_at)
		VALUES (9001, 'Sesion Backfill', 9001, now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO group_calendar_days (id, group_id, date, session_instance_id, kind, created_at, updated_at)
		VALUES (9001, 9001, '2026-09-01', 9001, 'asynchronous', now(), now())`).Error)

	mk := func(id int64, name string, source *int64) int64 {
		require.NoError(t, db.Exec(`INSERT INTO exercise_instances (id, name, kind, minutes, source_exercise_id, created_at)
			VALUES (?, ?, 'ritmo_continuo', 30, ?, now())`, id, name, source).Error)
		require.NoError(t, db.Exec(`INSERT INTO session_exercise_instances (id, session_instance_id, exercise_instance_id, role)
			VALUES (?, 9001, ?, 'main')`, id*10, id).Error)
		return id
	}

	linkedID = mk(9001, "Distancia 4 km", func() *int64 { v := int64(9001); return &v }())
	// Tilde y punto: sin normalizar serían OTRO ejercicio del mismo dueño.
	accentID = mk(9002, "Distancia 4 km.", nil)
	// Sin nada en el catálogo que lo matchee.
	orphanID = mk(9003, "Ejercicio Borrado del Catalogo", nil)

	return ownerID, linkedID, accentID, orphanID
}

func TestBackfillSourceExerciseID_LinksByNormalizedName(t *testing.T) {
	db := testutils.SetupTestDB(t)
	_, linkedID, accentID, orphanID := seedBackfill(t, db)

	require.NoError(t, postgresdb.BackfillSourceExerciseID(db))

	var src *int64
	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = ?`, accentID).Scan(&src).Error)
	require.NotNil(t, src, "el nombre con punto y tilde tiene que linkear al ejercicio 9001 del mismo dueño")
	assert.Equal(t, int64(9001), *src)

	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = ?`, linkedID).Scan(&src).Error)
	require.NotNil(t, src, "la fila que ya tenía link no se toca")
	assert.Equal(t, int64(9001), *src)

	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = ?`, orphanID).Scan(&src).Error)
	assert.Nil(t, src, "sin match en el catálogo queda en NULL para el fallback, no se descarta")
}

func TestBackfillSourceExerciseID_IsIdempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	_, _, accentID, _ := seedBackfill(t, db)

	require.NoError(t, postgresdb.BackfillSourceExerciseID(db))
	var first *int64
	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = ?`, accentID).Scan(&first).Error)
	require.NotNil(t, first)

	// Segunda pasada: el WHERE filtra las NULL, no tiene que volver a escribir.
	require.NoError(t, postgresdb.BackfillSourceExerciseID(db))
	var second *int64
	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = ?`, accentID).Scan(&second).Error)
	require.NotNil(t, second)
	assert.Equal(t, *first, *second)
}

func TestBackfillSourceExerciseID_DoesNotCrossOwners(t *testing.T) {
	db := testutils.SetupTestDB(t)
	_, _, _, _ = seedBackfill(t, db)

	// Otro entrenador con un "Distancia 4 km" propio: el backfill del primero
	// no puede linkear las instancias del segundo, y viceversa.
	require.NoError(t, db.Exec(`INSERT INTO users (id, name, surname, email, password, dni, birth_date, status, created_at, updated_at)
		VALUES (9002, 'Otro', 'Owner', 'otro@backfill.test', 'x', '39000002', '1990-01-01', 'active', now(), now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO exercises (id, owner_id, name, kind, minutes, distance_m, created_at, updated_at)
		VALUES (9003, 9002, 'Distancia 4 km', 'ritmo_continuo', 99, 4000, now(), now())`).Error)

	require.NoError(t, postgresdb.BackfillSourceExerciseID(db))

	// La instancia del owner 9001 tiene que seguir apuntando a SU ejercicio (9001),
	// no al del otro dueño (9003) aunque el nombre normalizado sea idéntico.
	var src *int64
	require.NoError(t, db.Raw(`SELECT source_exercise_id FROM exercise_instances WHERE id = 9002`).Scan(&src).Error)
	require.NotNil(t, src)
	assert.Equal(t, int64(9001), *src)
}
