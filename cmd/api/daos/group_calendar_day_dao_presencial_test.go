package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// día instanciado: instancia + día de calendario que la referencia (Gap 26).
func presencialDaySeed(t *testing.T, db *gorm.DB, tag string, isPresencial bool) (*dbs.SessionInstance, *dbs.GroupCalendarDay) {
	t.Helper()
	group := setupCalendarGroup(t, db, tag)
	inst := &dbs.SessionInstance{Name: "Fartlek " + tag}
	require.NoError(t, db.Create(inst).Error)
	day := &dbs.GroupCalendarDay{
		GroupID:           group.ID,
		Date:              time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC),
		Kind:              "training",
		SessionInstanceID: &inst.ID,
		IsPresencial:      isPresencial,
	}
	require.NoError(t, db.Create(day).Error)
	return inst, day
}

func TestGroupCalendarDayDao_FindBySessionInstanceID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst, day := presencialDaySeed(t, db, "fbi1", true)

	found, err := dao.FindBySessionInstanceID(nil, inst.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, day.ID, found.ID)
}

func TestGroupCalendarDayDao_FindWithTeamOwner_ResuelveDiaYOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "fwo")
	inst := &dbs.SessionInstance{Name: "con owner"}
	require.NoError(t, db.Create(inst).Error)
	day := &dbs.GroupCalendarDay{
		GroupID:           group.ID,
		Date:              time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC),
		Kind:              "training",
		SessionInstanceID: &inst.ID,
		IsPresencial:      true,
	}
	require.NoError(t, db.Create(day).Error)
	var team dbs.Team
	require.NoError(t, db.First(&team, group.TeamID).Error)

	found, ownerID, err := dao.FindWithTeamOwnerBySessionInstanceID(nil, inst.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, day.ID, found.ID)
	require.NotNil(t, ownerID)
	assert.Equal(t, team.OwnerID, *ownerID)
}

func TestGroupCalendarDayDao_FindWithTeamOwner_Orphan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)

	found, ownerID, err := dao.FindWithTeamOwnerBySessionInstanceID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
	assert.Nil(t, ownerID)
}

// Grupo o team soft-deleted conservan el día pero sin owner (mismo resultado
// que los FindByID separados que reemplaza).
func TestGroupCalendarDayDao_FindWithTeamOwner_SoftDeleted(t *testing.T) {
	cases := []struct {
		name      string
		deleteSQL string
	}{
		{"group", "UPDATE groups SET deleted_at = NOW() WHERE id = ?"},
		{"team", "UPDATE teams SET deleted_at = NOW() WHERE id = ?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			dao := NewGroupCalendarDayDao(db)
			group := setupCalendarGroup(t, db, "fws"+tc.name)
			inst := &dbs.SessionInstance{Name: "soft " + tc.name}
			require.NoError(t, db.Create(inst).Error)
			day := &dbs.GroupCalendarDay{
				GroupID:           group.ID,
				Date:              time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC),
				Kind:              "training",
				SessionInstanceID: &inst.ID,
				IsPresencial:      true,
			}
			require.NoError(t, db.Create(day).Error)

			target := group.ID
			if tc.name == "team" {
				target = group.TeamID
			}
			require.NoError(t, db.Exec(tc.deleteSQL, target).Error)

			found, ownerID, err := dao.FindWithTeamOwnerBySessionInstanceID(nil, inst.ID)

			require.NoError(t, err)
			require.NotNil(t, found, "el día se conserva")
			assert.Nil(t, ownerID)
		})
	}
}

func TestGroupCalendarDayDao_FindBySessionInstanceID_Orphan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst := &dbs.SessionInstance{Name: "huérfana sin día"}
	require.NoError(t, db.Create(inst).Error)

	found, err := dao.FindBySessionInstanceID(nil, inst.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupCalendarDayDao_SetPresencialOpenedAt(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbo1", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)

	updated, mutated, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)
	assert.True(t, mutated)
	require.NotNil(t, updated, "RETURNING entrega la fila post-write")
	require.NotNil(t, updated.PresencialOpenedAt)
	assert.Equal(t, at.UTC(), updated.PresencialOpenedAt.UTC())

	// Guard: ya abierta → no re-escribe ni marca mutación (idempotente).
	updated2, mutated2, err := dao.SetPresencialOpenedAt(nil, day.ID, at.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, mutated2)
	assert.Nil(t, updated2)

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialOpenedAt)
	assert.Equal(t, at.UTC(), found.PresencialOpenedAt.UTC())
	require.Nil(t, found.PresencialClosedAt)
}

func TestGroupCalendarDayDao_SetPresencialClosedAt(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbc1", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	_, _, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)

	updated, mutated, err := dao.SetPresencialClosedAt(nil, day.ID, closed)
	require.NoError(t, err)
	assert.True(t, mutated)
	require.NotNil(t, updated)
	require.NotNil(t, updated.PresencialClosedAt)
	assert.Equal(t, closed.UTC(), updated.PresencialClosedAt.UTC())

	updated2, mutated2, err := dao.SetPresencialClosedAt(nil, day.ID, closed.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, mutated2, "cierre final: sin re-escritura")
	assert.Nil(t, updated2)

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialClosedAt)
	assert.Equal(t, closed.UTC(), found.PresencialClosedAt.UTC())
}

func TestGroupCalendarDayDao_SetPresencial_DayInexistente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)

	updated, mutated, err := dao.SetPresencialOpenedAt(nil, 999999999, at)
	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, updated)

	updated, mutated, err = dao.SetPresencialClosedAt(nil, 999999999, at)
	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, updated)
}

// Gap 26 D5: reasignar el día a otra instancia reinicia el estado presencial;
// editar el mismo día con la MISMA instancia lo conserva.
func TestGroupCalendarDayDao_Upsert_ReasignacionReseteaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu1", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, _, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)

	newInst := *day.SessionInstanceID + 500
	require.NoError(t, db.Create(&dbs.SessionInstance{ID: newInst, Name: "otra"}).Error)
	reassigned := &dbs.GroupCalendarDay{
		GroupID:           day.GroupID,
		Date:              day.Date,
		Kind:              day.Kind,
		SessionInstanceID: &newInst,
	}
	require.NoError(t, dao.Upsert(nil, reassigned))

	found, findErr := dao.FindBySessionInstanceID(nil, newInst)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Nil(t, found.PresencialOpenedAt, "día reasignado arranca sin apertura")
}

func TestGroupCalendarDayDao_Upsert_MismaInstanciaConservaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu2", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, _, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)

	// Re-upsert con la misma instancia y el día aún presencial conserva la apertura.
	edited := &dbs.GroupCalendarDay{
		GroupID:           day.GroupID,
		Date:              day.Date,
		Kind:              day.Kind,
		SessionInstanceID: day.SessionInstanceID,
		IsPresencial:      true,
	}
	require.NoError(t, dao.Upsert(nil, edited))

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialOpenedAt, "edición con misma instancia no borra la apertura")
	assert.Equal(t, at.UTC(), (*found.PresencialOpenedAt).UTC())
}

// D5: apagar is_presencial limpia opened/closed — "no presencial = sin estado".
func TestGroupCalendarDayDao_Upsert_ApagaPresencialLimpiaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu3", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	_, _, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)
	_, _, err = dao.SetPresencialClosedAt(nil, day.ID, closed)
	require.NoError(t, err)

	off := &dbs.GroupCalendarDay{
		GroupID:           day.GroupID,
		Date:              day.Date,
		Kind:              day.Kind,
		SessionInstanceID: day.SessionInstanceID,
		IsPresencial:      false,
	}
	require.NoError(t, dao.Upsert(nil, off))

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.False(t, found.IsPresencial)
	assert.Nil(t, found.PresencialOpenedAt, "apagar presencial limpia la apertura")
	assert.Nil(t, found.PresencialClosedAt, "apagar presencial limpia el cierre")
}

// D5: re-activar presencial sobre un día cuyo estado ya se limpia arranca sin
// apertura/cierre (nueva apertura obligatoria).
func TestGroupCalendarDayDao_Upsert_ReactivarPresencial_ArrancaSinEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu4", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, _, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)

	off := &dbs.GroupCalendarDay{GroupID: day.GroupID, Date: day.Date, Kind: day.Kind, SessionInstanceID: day.SessionInstanceID}
	require.NoError(t, dao.Upsert(nil, off))
	on := &dbs.GroupCalendarDay{GroupID: day.GroupID, Date: day.Date, Kind: day.Kind, SessionInstanceID: day.SessionInstanceID, IsPresencial: true}
	require.NoError(t, dao.Upsert(nil, on))

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.True(t, found.IsPresencial)
	assert.Nil(t, found.PresencialOpenedAt, "re-activar presencial no revive la apertura vieja")
	assert.Nil(t, found.PresencialClosedAt)
}

// AutoMigrate debe crear el índice sobre session_instance_id (FindBySessionInstanceID
// y las colisiones resuelven el día por esa columna).
func TestGroupCalendarDayDao_SessionInstanceIDIndexExists(t *testing.T) {
	db := testutils.SetupTestDB(t)

	var n int64
	err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE tablename = 'group_calendar_days' AND indexname = 'idx_group_calendar_days_session_instance_id'`).Scan(&n).Error

	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
}
