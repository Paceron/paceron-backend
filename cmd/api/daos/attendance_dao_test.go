package daos

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewAttendanceDao(t *testing.T) {
	dao := NewAttendanceDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestAttendanceDao_ImplementsInterface(t *testing.T) {
	dao := NewAttendanceDao(&gorm.DB{})
	var iface AttendanceDAOInterface = dao
	_ = iface
}

// testSessionInstance crea una fila real en session_instances y devuelve su id.
//
// Hace falta porque la migración de este change agrega la FK
// attendances.training_session_id -> session_instances.id: si los tests usaran un
// id inventado (1, 2, 3) como antes, todos empezarían a fallar con violación de FK
// apenas la migración se aplique.
func testSessionInstance(db *gorm.DB, name string) int64 {
	si := &dbs.SessionInstance{Name: name}
	if err := db.Create(si).Error; err != nil {
		panic("no se pudo crear el session_instance de test: " + err.Error())
	}
	return si.ID
}

// newTestAttendance arma una asistencia lista para insertar.
//
// Source va explícito porque la migración lo deja NOT NULL: los fixtures que no lo
// seteen fallarían con violación de NOT NULL igual que la app.
func newTestAttendance(db *gorm.DB, teamID, sessionID, userID int64) *dbs.Attendance {
	return &dbs.Attendance{
		TeamID:            teamID,
		TrainingSessionID: sessionID,
		UserID:            userID,
		Source:            string(constants.AttendanceSourceQR),
	}
}

// testAttendance crea y guarda una asistencia directa por GORM (bypass del DAO).
func testAttendance(db *gorm.DB, teamID, sessionID, userID int64) *dbs.Attendance {
	att := newTestAttendance(db, teamID, sessionID, userID)
	db.Create(att)
	return att
}

func TestAttendanceDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)

	att := newTestAttendance(db, 1, testSessionInstance(db, "sesion-qr"), 1)
	err := dao.Create(nil, att)

	require.NoError(t, err)
	assert.NotZero(t, att.ID)
}

func TestAttendanceDao_Create_Duplicate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)

	att := &dbs.Attendance{TeamID: 1, TrainingSessionID: 1, UserID: 1}
	require.NoError(t, dao.Create(nil, att))

	dup := &dbs.Attendance{TeamID: 1, TrainingSessionID: 1, UserID: 1}
	err := dao.Create(nil, dup)

	require.ErrorIs(t, err, ErrAttendanceAlreadyExists)
}

func TestAttendanceDao_Create_SameUserDifferentSession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)

	require.NoError(t, dao.Create(nil, newTestAttendance(db, 1, testSessionInstance(db, "sesion-a"), 1)))
	err := dao.Create(nil, newTestAttendance(db, 1, testSessionInstance(db, "sesion-b"), 1))

	require.NoError(t, err)
}

func TestAttendanceDao_Search_NoFilters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	testAttendance(db, 1, 1, 1)
	testAttendance(db, 1, 2, 1)
	testAttendance(db, 2, 1, 2)

	atts, err := dao.Search(nil, AttendanceSearchFilters{})

	require.NoError(t, err)
	assert.Len(t, atts, 3)
}

func TestAttendanceDao_Search_FilterByTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	testAttendance(db, 1, 1, 1)
	testAttendance(db, 1, 2, 1)
	testAttendance(db, 2, 1, 2)

	teamID := int64(2)
	atts, err := dao.Search(nil, AttendanceSearchFilters{TeamID: &teamID})

	require.NoError(t, err)
	require.Len(t, atts, 1)
	assert.Equal(t, int64(2), atts[0].TeamID)
}

func TestAttendanceDao_Search_FilterBySession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	testAttendance(db, 1, 1, 1)
	testAttendance(db, 1, 2, 1)

	sessionID := int64(1)
	atts, err := dao.Search(nil, AttendanceSearchFilters{TrainingSessionID: &sessionID})

	require.NoError(t, err)
	require.Len(t, atts, 1)
	assert.Equal(t, int64(1), atts[0].TrainingSessionID)
}

func TestAttendanceDao_Search_FilterByUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	testAttendance(db, 1, 1, 1)
	testAttendance(db, 1, 2, 2)

	userID := int64(2)
	atts, err := dao.Search(nil, AttendanceSearchFilters{UserID: &userID})

	require.NoError(t, err)
	require.Len(t, atts, 1)
	assert.Equal(t, int64(2), atts[0].UserID)
}

func TestAttendanceDao_Search_AllFilters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	testAttendance(db, 1, 1, 1)
	testAttendance(db, 1, 1, 2)
	testAttendance(db, 1, 2, 1)
	testAttendance(db, 2, 1, 1)

	teamID := int64(1)
	sessionID := int64(1)
	userID := int64(2)
	atts, err := dao.Search(nil, AttendanceSearchFilters{TeamID: &teamID, TrainingSessionID: &sessionID, UserID: &userID})

	require.NoError(t, err)
	require.Len(t, atts, 1)
	assert.Equal(t, int64(1), atts[0].TeamID)
	assert.Equal(t, int64(1), atts[0].TrainingSessionID)
	assert.Equal(t, int64(2), atts[0].UserID)
}

func TestAttendanceDao_Search_NoMatches(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)

	teamID := int64(99)
	atts, err := dao.Search(nil, AttendanceSearchFilters{TeamID: &teamID})

	require.NoError(t, err)
	assert.Empty(t, atts)
}

func TestAttendanceDao_TeamExists(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	owner := persistUser(db, "att-team-exists-owner@test.com", "30000001")
	team := testTeam(db, "att_exists_team", owner.ID)

	exists, err := dao.TeamExists(nil, team.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = dao.TeamExists(nil, 99999)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestAttendanceDao_IsTeamOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	owner := persistUser(db, "att-owner@test.com", "30000002")
	other := persistUser(db, "att-other@test.com", "30000003")
	team := testTeam(db, "att_owner_team", owner.ID)

	isOwner, err := dao.IsTeamOwner(nil, team.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isOwner)

	isOwner, err = dao.IsTeamOwner(nil, team.ID, other.ID)
	require.NoError(t, err)
	assert.False(t, isOwner)

	isOwner, err = dao.IsTeamOwner(nil, 99999, owner.ID)
	require.NoError(t, err)
	assert.False(t, isOwner)
}

func TestAttendanceDao_ExistsUserInTeamOwnedBy(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	owner := persistUser(db, "att-owner2@test.com", "30000004")
	member := persistUser(db, "att-member@test.com", "30000005")
	team := testTeam(db, "att_owned_team", owner.ID)
	db.Create(&dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()})

	found, err := dao.ExistsUserInTeamOwnedBy(nil, member.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestAttendanceDao_ExistsUserInTeamOwnedBy_NotMember(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	owner := persistUser(db, "att-owner3@test.com", "30000006")
	outsider := persistUser(db, "att-outsider@test.com", "30000007")
	persistUser(db, "att-owner3-member@test.com", "30000008")
	testTeam(db, "att_owned_team2", owner.ID)

	found, err := dao.ExistsUserInTeamOwnedBy(nil, outsider.ID, owner.ID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestAttendanceDao_ExistsUserInTeamOwnedBy_OwnerOfOtherTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	ownerA := persistUser(db, "att-ownera@test.com", "30000009")
	ownerB := persistUser(db, "att-ownerb@test.com", "30000010")
	member := persistUser(db, "att-member2@test.com", "30000011")
	teamB := testTeam(db, "att_team_b", ownerB.ID)
	db.Create(&dbs.TeamUser{TeamID: teamB.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()})

	found, err := dao.ExistsUserInTeamOwnedBy(nil, member.ID, ownerA.ID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestAttendanceDao_GetTeamUserRole(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	coach := persistUser(db, "att-coach3@test.com", "30000012")
	runner := persistUser(db, "att-runner3@test.com", "30000013")
	team := testTeam(db, "att_role_team", coach.ID)
	db.Create(&dbs.TeamUser{TeamID: team.ID, UserID: coach.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()})
	db.Create(&dbs.TeamUser{TeamID: team.ID, UserID: runner.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()})

	role, err := dao.GetTeamUserRole(nil, team.ID, coach.ID)
	require.NoError(t, err)
	assert.Equal(t, "entrenador", role)

	role, err = dao.GetTeamUserRole(nil, team.ID, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, "corredor", role)
}

func TestAttendanceDao_GetTeamUserRole_NotMember(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	coach := persistUser(db, "att-coach4@test.com", "30000014")
	outside := persistUser(db, "att-outside4@test.com", "30000015")
	team := testTeam(db, "att_role_team2", coach.ID)
	db.Create(&dbs.TeamUser{TeamID: team.ID, UserID: coach.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()})

	role, err := dao.GetTeamUserRole(nil, team.ID, outside.ID)
	require.NoError(t, err)
	assert.Equal(t, "", role)
}

// TestAttendanceDao_UniqueIndexExists verifica la precondición del ON CONFLICT de
// BulkUpsertManual.
//
// Esta es la única verificación que importa para ese método, y no se puede hacer
// sin Postgres. El `ON CONFLICT (team_id, training_session_id, user_id)` no nombra
// un índice: Postgres busca uno UNIQUE y NO PARCIAL sobre esas columnas, en ese
// orden. Si no lo encuentra, la sentencia falla en runtime — no al compilar, no al
// correr los otros tests, sino la primera vez que un entrenador guarda asistencia.
//
// La consulta mira pg_indexes y no el modelo GORM a propósito: el tag
// `uniqueIndex:uq_att_team_session_user` del struct describe lo que GORM crearía con
// AutoMigrate sobre una base nueva, y no es prueba de nada sobre la base real.
func TestAttendanceDao_UniqueIndexExists(t *testing.T) {
	db := testutils.SetupTestDB(t)

	var defs []string
	err := db.Raw(`
		SELECT indexdef
		FROM pg_indexes
		WHERE tablename = 'attendances'
		  AND indexdef ILIKE '%UNIQUE%'
		  AND indexdef ~ 'team_id.*training_session_id.*user_id'
	`).Scan(&defs).Error

	require.NoError(t, err)
	require.NotEmpty(t, defs,
		"no hay índice UNIQUE sobre (team_id, training_session_id, user_id) en attendances: "+
			"BulkUpsertManual fallaría en runtime. Crear con: "+
			"CREATE UNIQUE INDEX uq_att_team_session_user ON attendances (team_id, training_session_id, user_id)")

	// Se exige que AL MENOS uno sea utilizable, en vez de mirar solo el primero.
	// Puede haber más de un índice sobre esas columnas (alguien pudo crear uno
	// redundante), y el `ON CONFLICT` matchea con cualquiera que sirva: si el
	// primero alfabéticamente fuera parcial pero hubiera otro completo, el
	// código funcionaría y un test que mirara solo el primero fallaría en falso.
	var usable []string
	for _, def := range defs {
		if !strings.Contains(def, " WHERE ") {
			usable = append(usable, def)
		}
	}
	require.NotEmpty(t, usable,
		"los únicos índices sobre esas columnas son PARCIALES: Register funcionaría (un índice parcial "+
			"también lanza 23505) pero el ON CONFLICT de BulkUpsertManual fallaría. "+
			"Crear uno completo: CREATE UNIQUE INDEX uq_att_team_session_user "+
			"ON attendances (team_id, training_session_id, user_id). Índices hallados: %v", defs)
}

// TestAttendanceDao_BulkUpsertManual_Idempotencia es el test de 3.7: los 3 casos
// de contadores que pide la spec, más que no se acumulen duplicados.
func TestAttendanceDao_BulkUpsertManual_Idempotencia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	const (
		teamID  = int64(1)
		actorID = int64(99)
	)
	sessionID := testSessionInstance(db, "sesion-bulk")

	t.Run("3 inserts nuevos dan created:3 updated:0", func(t *testing.T) {
		created, updated, err := dao.BulkUpsertManual(nil, teamID, sessionID, actorID, []int64{10, 11, 12})

		require.NoError(t, err)
		assert.Equal(t, 3, created)
		assert.Equal(t, 0, updated)

		var rows int64
		require.NoError(t, db.Model(&dbs.Attendance{}).
			Where("team_id = ? AND training_session_id = ?", teamID, sessionID).Count(&rows).Error)
		assert.Equal(t, int64(3), rows)
	})

	t.Run("el mismo lote de nuevo da created:0 updated:3 y no duplica", func(t *testing.T) {
		created, updated, err := dao.BulkUpsertManual(nil, teamID, sessionID, actorID, []int64{10, 11, 12})

		require.NoError(t, err)
		assert.Equal(t, 0, created)
		assert.Equal(t, 3, updated)

		var rows int64
		require.NoError(t, db.Model(&dbs.Attendance{}).
			Where("team_id = ? AND training_session_id = ?", teamID, sessionID).Count(&rows).Error)
		assert.Equal(t, int64(3), rows, "un upsert idempotente nunca acumula filas")
	})

	t.Run("lote mixto da created:1 updated:1", func(t *testing.T) {
		created, updated, err := dao.BulkUpsertManual(nil, teamID, sessionID, actorID, []int64{12, 13})

		require.NoError(t, err)
		assert.Equal(t, 1, created)
		assert.Equal(t, 1, updated)

		var rows int64
		require.NoError(t, db.Model(&dbs.Attendance{}).
			Where("team_id = ? AND training_session_id = ?", teamID, sessionID).Count(&rows).Error)
		assert.Equal(t, int64(4), rows)
	})

	t.Run("escribe source y registered_by del actor", func(t *testing.T) {
		var got dbs.Attendance
		require.NoError(t, db.Where("team_id = ? AND training_session_id = ? AND user_id = ?", teamID, sessionID, 13).
			First(&got).Error)

		assert.Equal(t, string(constants.AttendanceSourceManual), got.Source)
		require.NotNil(t, got.RegisteredByUserID)
		assert.Equal(t, actorID, *got.RegisteredByUserID)
	})

	t.Run("un lote vacio es un no-op que no toca la DB", func(t *testing.T) {
		created, updated, err := dao.BulkUpsertManual(nil, teamID, sessionID, actorID, []int64{})

		require.NoError(t, err)
		assert.Equal(t, 0, created)
		assert.Equal(t, 0, updated)
	})
}

func TestAttendanceDao_FindByID_And_DeleteByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	sessionID := testSessionInstance(db, "sesion-borrado")
	att := newTestAttendance(db, 2, sessionID, 20)
	require.NoError(t, dao.Create(nil, att))

	t.Run("FindByID devuelve la fila", func(t *testing.T) {
		got, err := dao.FindByID(nil, att.ID)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, att.ID, got.ID)
	})

	t.Run("FindByID de un id inexistente devuelve nil sin error", func(t *testing.T) {
		got, err := dao.FindByID(nil, att.ID+99999)

		require.NoError(t, err, "no es un error de la app: el service lo mapea a 404")
		assert.Nil(t, got)
	})

	t.Run("DeleteByID borra y devuelve true", func(t *testing.T) {
		deleted, err := dao.DeleteByID(nil, att.ID, 2)

		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("DeleteByID de un id inexistente devuelve false sin error", func(t *testing.T) {
		deleted, err := dao.DeleteByID(nil, att.ID, 2)

		require.NoError(t, err)
		assert.False(t, deleted)
	})

	t.Run("DeleteByID con otro team_id NO borra", func(t *testing.T) {
		other := newTestAttendance(db, 3, sessionID, 21)
		require.NoError(t, dao.Create(nil, other))

		deleted, err := dao.DeleteByID(nil, other.ID, 999)

		require.NoError(t, err)
		assert.False(t, deleted, "el team_id va en el WHERE: es la condición de seguridad de la sentencia")

		got, err := dao.FindByID(nil, other.ID)
		require.NoError(t, err)
		assert.NotNil(t, got, "la fila de otro equipo sigue ahí")
	})
}

// TestAttendanceDao_FindGroupRosterWithAttendance_MembershipWindow cubre (a), (b) y
// (c) de la tarea 2.9: el denominador de la tasa de asistencia se evalúa contra la
// FECHA DE LA SESIÓN, no contra la fecha en que se consulta.
//
// El bug que esto evita es sutil y casi invisible. Si la query filtrara por NOW()
// en vez de por sessionDate, todo funcionaría bien en el momento de cargarla y
// empezaría a mentir días después, sin ningún error: la membresía de un corredor
// que venció DESPUÉS de la clase se seguiría contando hoy, y la de alguien que se
// sumó después ya no, para una sesión que ya ocurrió. El % de asistencia se movería
// solo con el paso del tiempo, sobre datos que no cambiaron.
func TestAttendanceDao_FindGroupRosterWithAttendance_MembershipWindow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)
	guDao := NewGroupUserDao(db)

	owner := persistUser(db, "roster-window-owner@test.com", "53000001")
	team := testTeam(db, "equipo_roster_window", owner.ID)
	group := testGroup(db, "grupo_roster_window", team.ID)
	sessionID := testSessionInstance(db, "sesion-roster-window")

	sessionDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	before := sessionDate.AddDate(0, 0, -30)
	after := sessionDate.AddDate(0, 0, 30)

	addMember := func(suffix, dni string, start time.Time, end *time.Time) int64 {
		u := persistUser(db, "roster-window-"+suffix+"@test.com", dni)
		require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
			GroupID: group.ID, UserID: u.ID, DateStart: start, DateEnd: end,
		}))
		return u.ID
	}

	stillActive := addMember("activo", "53000002", before, nil)
	endsLater := addMember("vencido-despues", "53000003", before, &after)
	endedBefore := addMember("vencido-antes", "53000004", before, &before)
	notYet := addMember("sumado-despues", "53000005", after, nil)

	// rosterExpected son los 2 que la ventana cubre, ordenados por user_id como
	// ordena la query. Quedan afuera endedBefore (date_end anterior a la sesión) y
	// notYet (date_start posterior).
	rosterExpected := []int64{stillActive, endsLater}
	sort.Slice(rosterExpected, func(i, j int) bool { return rosterExpected[i] < rosterExpected[j] })

	t.Run("(a) roster_size cuenta los vigentes en la fecha de la sesion", func(t *testing.T) {
		rows, aggregates, err := dao.FindGroupRosterWithAttendance(nil, group.ID, team.ID, sessionID, sessionDate)

		require.NoError(t, err)
		// De los 4, entran 2: date_end ANTERIOR y date_start POSTERIOR quedan
		// afuera; date_end POSTERIOR entra porque la ventana es inclusiva (>=).
		assert.Equal(t, int64(2), aggregates.RosterSize)

		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
			assert.Nil(t, r.AttendanceID, "todavía nadie asistió")
		}
		assert.Equal(t, rosterExpected, ids, "mismo set y mismo orden que ORDER BY user_id")
	})

	t.Run("(b) las membresias que no cubren la sesion quedan afuera", func(t *testing.T) {
		rows, aggregates, err := dao.FindGroupRosterWithAttendance(nil, group.ID, team.ID, sessionID, sessionDate)

		require.NoError(t, err)
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		assert.NotContains(t, ids, endedBefore, "date_end anterior a la sesión")
		assert.NotContains(t, ids, notYet, "date_start posterior a la sesión")
		assert.Equal(t, int64(2), aggregates.RosterSize)
	})

	t.Run("(c) el mismo sessionDate da el mismo roster_size aunque la membresia venza despues", func(t *testing.T) {
		// Simula el paso del tiempo: la membresía de stillActive pasa a estar
		// vencida HOY, después de que la sesión ya ocurrió. Con la query bien
		// escrita sigue contando, porque lo que manda es sessionDate. Con NOW()
		// el denominador bajaría de 3 a 2 y la tasa de asistencia saltaría para
		// una sesión cuyos datos no cambiaron.
		today := time.Now().UTC().Truncate(24 * time.Hour)
		require.NoError(t, db.Model(&dbs.GroupUser{}).
			Where("group_id = ? AND user_id = ?", group.ID, stillActive).
			Update("date_end", today).Error)

		rows, aggregates, err := dao.FindGroupRosterWithAttendance(nil, group.ID, team.ID, sessionID, sessionDate)

		require.NoError(t, err)
		assert.Equal(t, int64(2), aggregates.RosterSize,
			"el denominador depende de sessionDate, no de la fecha en que se consulta")
		assert.Len(t, rows, 2)
	})

	t.Run("attended cuenta asistencias de gente fuera del roster vigente", func(t *testing.T) {
		// El total de asistentes es de la SESIÓN, no del roster. Un corredor cuya
		// membresía ya había vencido cuando se dio la clase puede igual haber
		// asistido: si attended se filtrara por el roster, el porcentaje podría
		// dar más de 100%.
		require.NoError(t, dao.Create(nil, &dbs.Attendance{
			TeamID: team.ID, TrainingSessionID: sessionID, UserID: endedBefore,
			Source: string(constants.AttendanceSourceQR),
		}))

		rows, aggregates, err := dao.FindGroupRosterWithAttendance(nil, group.ID, team.ID, sessionID, sessionDate)

		require.NoError(t, err)
		assert.Equal(t, int64(1), aggregates.Attended)
		assert.Equal(t, int64(2), aggregates.RosterSize, "el roster no cambia por la asistencia")
		// El que asiste fuera del roster no aparece en las filas, pero sí suma.
		for _, r := range rows {
			assert.NotEqual(t, endedBefore, r.UserID)
		}
	})
}
