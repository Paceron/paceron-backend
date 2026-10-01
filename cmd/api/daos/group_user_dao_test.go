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

func TestNewGroupUserDao(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestGroupUserDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	var iface GroupUserDaoInterface = dao
	_ = iface
}

func TestGroupUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewGroupUserDao(&gorm.DB{})
	})
}

func TestGroupUserDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-create-owner@test.com", "50000001")
	team := testTeam(db, "equipo_gu_create", owner.ID)
	group := testGroup(db, "grupo_gu_create", team.ID)

	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	err := dao.Create(nil, gu)

	require.NoError(t, err)
	assert.NotZero(t, gu.ID)
}

func TestGroupUserDao_FindByGroupAndUser_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-find-owner@test.com", "50000002")
	team := testTeam(db, "equipo_gu_find", owner.ID)
	group := testGroup(db, "grupo_gu_find", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	found, err := dao.FindByGroupAndUser(nil, group.ID, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, gu.ID, found.ID)
}

func TestGroupUserDao_FindByGroupAndUser_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)

	found, err := dao.FindByGroupAndUser(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupUserDao_FindByGroupID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-bygroup-owner@test.com", "50000003")
	member := persistUser(db, "gu-bygroup-member@test.com", "50000004")
	team := testTeam(db, "equipo_gu_bygroup", owner.ID)
	group := testGroup(db, "grupo_gu_bygroup", team.ID)

	active := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	deleted := &dbs.GroupUser{GroupID: group.ID, UserID: member.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByGroupID(nil, group.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestGroupUserDao_FindByUserID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-byuser-owner@test.com", "50000005")
	team := testTeam(db, "equipo_gu_byuser", owner.ID)
	group1 := testGroup(db, "grupo_gu_byuser1", team.ID)
	group2 := testGroup(db, "grupo_gu_byuser2", team.ID)

	active := &dbs.GroupUser{GroupID: group1.ID, UserID: owner.ID, DateStart: time.Now()}
	deleted := &dbs.GroupUser{GroupID: group2.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByUserID(nil, owner.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestGroupUserDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-softdelete-owner@test.com", "50000006")
	team := testTeam(db, "equipo_gu_softdelete", owner.ID)
	group := testGroup(db, "grupo_gu_softdelete", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	err := dao.SoftDelete(nil, gu.ID)

	require.NoError(t, err)
	found, _ := dao.FindByGroupAndUser(nil, group.ID, owner.ID)
	assert.Nil(t, found)
}

func TestGroupUserDao_SoftDeleteByTeamID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-softdeleteteam-owner@test.com", "50000007")
	team := testTeam(db, "equipo_gu_softdeleteteam", owner.ID)
	group := testGroup(db, "grupo_gu_softdeleteteam", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	err := dao.SoftDeleteByTeamID(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupID(nil, group.ID)
	require.NoError(t, findErr)
	assert.Empty(t, found)
}

// TestGroupUserDao_ActiveMembershipWindow cubre los 3 casos de D7: la membresía de
// un corredor se evalúa contra la FECHA DE LA SESIÓN, no contra hoy. Sin esto, la
// grilla mostraría "no pertenece al grupo" a alguien que sí estaba cuando se dio
// la clase, o al revés: dejaría marcar asistencia a quien ya se había ido.
//
// La ventana de una membresía es [date_start, date_end] con date_end nullable
// (null = sigue activo) y deleted_at nullable (null = no borrada lógicamente).
func TestGroupUserDao_ActiveMembershipWindow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-membership-owner@test.com", "51000001")
	team := testTeam(db, "equipo_gu_membership", owner.ID)
	group := testGroup(db, "grupo_gu_membership", team.ID)

	// La sesión cae en este día; todas las membresías se expresan relativas a él.
	sessionDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	before := sessionDate.AddDate(0, 0, -30)
	after := sessionDate.AddDate(0, 0, 30)
	deletedAt := sessionDate.AddDate(0, 0, -1)

	newUser := func(suffix, dni string) *dbs.User {
		return persistUser(db, "gu-membership-"+suffix+"@test.com", dni)
	}
	membership := func(userID int64, start time.Time, end *time.Time) {
		require.NoError(t, dao.Create(nil, &dbs.GroupUser{
			GroupID: group.ID, UserID: userID, DateStart: start, DateEnd: end,
		}))
	}
	softDeleted := func(userID int64, start time.Time) {
		gu := &dbs.GroupUser{GroupID: group.ID, UserID: userID, DateStart: start, DeletedAt: &deletedAt}
		require.NoError(t, db.Create(gu).Error)
	}

	stillActive := newUser("activo", "51000002")
	ended := newUser("terminada", "51000003")
	notYet := newUser("futuro", "51000004")
	logicallyDeleted := newUser("borrado", "51000005")
	endsThatDay := newUser("termina-ese-dia", "51000006")

	membership(stillActive.ID, before, nil)          // sigue en el grupo
	membership(ended.ID, before, &before)            // date_end antes de la sesión
	membership(notYet.ID, after, nil)                // se suma después
	membership(endsThatDay.ID, before, &sessionDate) // date_end exacto = día de la sesión
	softDeleted(logicallyDeleted.ID, before)         // borrado lógicamente antes

	t.Run("sigue activo: date_end NULL", func(t *testing.T) {
		got, err := dao.IsActiveGroupMember(nil, group.ID, stillActive.ID, sessionDate)

		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("date_end exacto el dia de la sesion cuenta como miembro", func(t *testing.T) {
		// La comparación es >=, no >: un corredor cuya membresía venció ESE día
		//，sigue habiendo asistido a la clase de ese día.
		got, err := dao.IsActiveGroupMember(nil, group.ID, endsThatDay.ID, sessionDate)

		require.NoError(t, err)
		assert.True(t, got, "date_end >= sessionDate: la ventana incluye el día de salida")
	})

	t.Run("membresia terminada antes de la sesion", func(t *testing.T) {
		got, err := dao.IsActiveGroupMember(nil, group.ID, ended.ID, sessionDate)

		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("todavia no era miembro en la sesion", func(t *testing.T) {
		got, err := dao.IsActiveGroupMember(nil, group.ID, notYet.ID, sessionDate)

		require.NoError(t, err)
		assert.False(t, got, "date_start > sessionDate: se suma después de la clase")
	})

	t.Run("borrado logicamente antes de la sesion", func(t *testing.T) {
		got, err := dao.IsActiveGroupMember(nil, group.ID, logicallyDeleted.ID, sessionDate)

		require.NoError(t, err)
		assert.False(t, got, "deleted_at no puede ser NULL para contar")
	})

	t.Run("un usuario de otro grupo no cuenta", func(t *testing.T) {
		otherTeam := testTeam(db, "equipo_gu_membership_otro", owner.ID)
		otherGroup := testGroup(db, "grupo_gu_membership_otro", otherTeam.ID)
		outsider := newUser("ajeno", "51000007")
		require.NoError(t, dao.Create(nil, &dbs.GroupUser{
			GroupID: otherGroup.ID, UserID: outsider.ID, DateStart: before,
		}))

		got, err := dao.IsActiveGroupMember(nil, group.ID, outsider.ID, sessionDate)

		require.NoError(t, err)
		assert.False(t, got, "membership del grupo ajeno no habilita en este")
	})
}

// TestGroupUserDao_MissingGroupMembers cubre el detector que usa la carga masiva:
// tiene que devolver SOLO los que no eran miembros, preservando el orden de
// entrada, porque ese listado va derecho a la respuesta 422.
func TestGroupUserDao_MissingGroupMembers(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-missing-owner@test.com", "52000001")
	team := testTeam(db, "equipo_gu_missing", owner.ID)
	group := testGroup(db, "grupo_gu_missing", team.ID)

	sessionDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	before := sessionDate.AddDate(0, 0, -30)

	member := persistUser(db, "gu-missing-miembro@test.com", "52000002")
	require.NoError(t, dao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: member.ID, DateStart: before,
	}))

	t.Run("lista solo los que no son miembros y conserva el orden", func(t *testing.T) {
		missing, err := dao.MissingGroupMembers(nil, group.ID, []int64{999, member.ID, 777}, sessionDate)

		require.NoError(t, err)
		assert.Equal(t, []int64{999, 777}, missing)
	})

	t.Run("lote vacio devuelve nil sin error", func(t *testing.T) {
		missing, err := dao.MissingGroupMembers(nil, group.ID, []int64{}, sessionDate)

		require.NoError(t, err)
		assert.Empty(t, missing)
	})

	t.Run("todos miembros devuelve lista vacia", func(t *testing.T) {
		missing, err := dao.MissingGroupMembers(nil, group.ID, []int64{member.ID}, sessionDate)

		require.NoError(t, err)
		assert.Empty(t, missing)
	})
}
