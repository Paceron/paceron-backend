package daos

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// accessFixture arma el escenario mínimo de session-instance-detail D2:
// owner→teamA→groupA con un día que referencia la instancia, un atleta
// miembro activo, un ex-miembro, y una segunda instancia huérfana para la
// rama de feedback.
type accessFixture struct {
	owner    *dbs.User // owner de teamA
	athlete  *dbs.User // miembro activo de groupA
	exMember *dbs.User // membresía vencida de groupA
	stranger *dbs.User // sin vínculos
	teamA    *dbs.Team
	groupA   *dbs.Group
	instDay  *dbs.SessionInstance // en el día de groupA
}

func accessFixtureSetup(t *testing.T, db *gorm.DB) *accessFixture {
	t.Helper()
	f := &accessFixture{
		owner:    persistUser(db, "access-owner@test.com", "70000001"),
		athlete:  persistUser(db, "access-athlete@test.com", "70000002"),
		exMember: persistUser(db, "access-exmember@test.com", "70000003"),
		stranger: persistUser(db, "access-stranger@test.com", "70000004"),
	}
	f.teamA = testTeam(db, "equipo_access", f.owner.ID)
	f.groupA = testGroup(db, "grupo_access", f.teamA.ID)

	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.groupA.ID, UserID: f.athlete.ID, DateStart: time.Now()}).Error)
	pastEnd := time.Now().AddDate(0, 0, -1)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.groupA.ID, UserID: f.exMember.ID, DateStart: time.Now().AddDate(0, 0, -30), DateEnd: &pastEnd}).Error)

	f.instDay = &dbs.SessionInstance{Name: "Instancia en día"}
	require.NoError(t, db.Create(f.instDay).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: f.groupA.ID, Date: time.Now(), Kind: "training", SessionInstanceID: &f.instDay.ID,
	}).Error)
	return f
}

func accessFeedback(t *testing.T, db *gorm.DB, athleteID, reporterID int64, teamID *int64, sessionID int64, deleted bool) *dbs.WorkoutFeedback {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	fb := &dbs.WorkoutFeedback{
		TeamID: teamID, AssignedSessionID: sessionID, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: reporterID,
		ReportSource: "corredor", SessionDate: time.Now(), SetNumber: 1, MediaURLs: media,
	}
	if deleted {
		now := time.Now()
		fb.DeletedAt = &now
	}
	require.NoError(t, db.Create(fb).Error)
	return fb
}

func TestSessionInstanceDao_HasInstanceAccess(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := accessFixtureSetup(t, db)
	dao := NewSessionInstanceDao(db)
	teamAID := f.teamA.ID

	t.Run("dia_miembro_activo", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, f.instDay.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("dia_owner_equipo", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, f.instDay.ID, f.owner.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("dia_ex_miembro_vencido", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, f.instDay.ID, f.exMember.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("dia_desconocido", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, f.instDay.ID, f.stranger.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("feedback_atleta", func(t *testing.T) {
		fbInst := &dbs.SessionInstance{Name: "Fb atleta"}
		require.NoError(t, db.Create(fbInst).Error)
		accessFeedback(t, db, f.athlete.ID, f.athlete.ID, &teamAID, fbInst.ID, false)
		ok, err := dao.HasInstanceAccess(nil, fbInst.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("feedback_reportante", func(t *testing.T) {
		fbInst := &dbs.SessionInstance{Name: "Fb reportante"}
		require.NoError(t, db.Create(fbInst).Error)
		reporter := persistUser(db, "access-reporter@test.com", "70000005")
		accessFeedback(t, db, f.athlete.ID, reporter.ID, &teamAID, fbInst.ID, false)
		ok, err := dao.HasInstanceAccess(nil, fbInst.ID, reporter.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("feedback_owner", func(t *testing.T) {
		fbInst := &dbs.SessionInstance{Name: "Fb owner"}
		require.NoError(t, db.Create(fbInst).Error)
		accessFeedback(t, db, f.athlete.ID, f.athlete.ID, &teamAID, fbInst.ID, false)
		ok, err := dao.HasInstanceAccess(nil, fbInst.ID, f.owner.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("feedback_soft_deleted_no_da_acceso", func(t *testing.T) {
		solo := &dbs.SessionInstance{Name: "Solo feedback borrado"}
		require.NoError(t, db.Create(solo).Error)
		accessFeedback(t, db, f.athlete.ID, f.athlete.ID, &teamAID, solo.ID, true)
		ok, err := dao.HasInstanceAccess(nil, solo.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("feedback_sin_equipo", func(t *testing.T) {
		solo := &dbs.SessionInstance{Name: "Feedback sin equipo"}
		require.NoError(t, db.Create(solo).Error)
		accessFeedback(t, db, f.athlete.ID, f.athlete.ID, nil, solo.ID, false)
		ok, err := dao.HasInstanceAccess(nil, solo.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("instancia_ajena_sin_vinculos", func(t *testing.T) {
		teamB := testTeam(db, "equipo_access_ajeno", f.stranger.ID)
		groupB := testGroup(db, "grupo_access_ajeno", teamB.ID)
		other := &dbs.SessionInstance{Name: "Del otro equipo"}
		require.NoError(t, db.Create(other).Error)
		require.NoError(t, db.Create(&dbs.GroupCalendarDay{
			GroupID: groupB.ID, Date: time.Now(), Kind: "training", SessionInstanceID: &other.ID,
		}).Error)
		ok, err := dao.HasInstanceAccess(nil, other.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
