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

func TestNewAttendanceDao(t *testing.T) {
	dao := NewAttendanceDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestAttendanceDao_ImplementsInterface(t *testing.T) {
	dao := NewAttendanceDao(&gorm.DB{})
	var iface AttendanceDAOInterface = dao
	_ = iface
}

// testAttendance crea y guarda una asistencia directa por GORM (bypass del DAO).
func testAttendance(db *gorm.DB, teamID, sessionID, userID int64) *dbs.Attendance {
	att := &dbs.Attendance{TeamID: teamID, TrainingSessionID: sessionID, UserID: userID}
	db.Create(att)
	return att
}

func TestAttendanceDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAttendanceDao(db)

	att := &dbs.Attendance{TeamID: 1, TrainingSessionID: 1, UserID: 1}
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

	require.NoError(t, dao.Create(nil, &dbs.Attendance{TeamID: 1, TrainingSessionID: 1, UserID: 1}))
	err := dao.Create(nil, &dbs.Attendance{TeamID: 1, TrainingSessionID: 2, UserID: 1})

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
