package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// detailUser crea un usuario válido con datos únicos (patrón exclOwnerGroup).
func detailUser(t *testing.T, db *gorm.DB, tag string) *dbs.User {
	t.Helper()
	u := &dbs.User{Name: tag, Surname: "Detalle", Email: tag + "@detail.test.com", DNI: "82" + tag}
	u.BirthDate = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	u.Password = "hashed"
	require.NoError(t, db.Create(u).Error)
	return u
}

// detailFixture arma el escenario mínimo de session-instance-detail: owner →
// team → group con un día que referencia la instancia, un atleta miembro
// activo, un ex-miembro, y feedbacks sobre instancias huérfanas.
type detailFixture struct {
	owner    *dbs.User
	athlete  *dbs.User
	exMember *dbs.User
	stranger *dbs.User
	team     *dbs.Team
	group    *dbs.Group
	instDay  *dbs.SessionInstance
	desc     string
}

func detailFixtureSetup(t *testing.T, db *gorm.DB, tag string) *detailFixture {
	t.Helper()
	f := &detailFixture{
		owner:    detailUser(t, db, "owner"+tag),
		athlete:  detailUser(t, db, "athlete"+tag),
		exMember: detailUser(t, db, "exm"+tag),
		stranger: detailUser(t, db, "str"+tag),
	}
	f.team = &dbs.Team{Name: "Team " + tag, MaxMembers: 10, OwnerID: f.owner.ID}
	require.NoError(t, db.Create(f.team).Error)
	f.group = &dbs.Group{Name: "Group " + tag, TeamID: f.team.ID, IsMain: true}
	require.NoError(t, db.Create(f.group).Error)

	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.group.ID, UserID: f.athlete.ID, DateStart: time.Now()}).Error)
	pastEnd := time.Now().AddDate(0, 0, -1)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.group.ID, UserID: f.exMember.ID, DateStart: time.Now().AddDate(0, 0, -30), DateEnd: &pastEnd}).Error)

	f.desc = "Fartlek congelado"
	f.instDay = &dbs.SessionInstance{Name: "Fartlek 5K", Description: &f.desc}
	require.NoError(t, db.Create(f.instDay).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: f.group.ID, Date: time.Now(), Kind: "training", SessionInstanceID: &f.instDay.ID,
	}).Error)
	return f
}

func detailSvc(db *gorm.DB) CalendarServiceInterface {
	return NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
}

// detailFeedback crea un workout_feedback con el mínimo exigido por la DB
// (MediaURLs es un pgtype.TextArray que necesita Set(nil) explícito).
func detailFeedback(t *testing.T, db *gorm.DB, athleteID, sessionID, exerciseID int64, teamID *int64) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID: teamID, AssignedSessionID: sessionID, AssignedExerciseID: exerciseID,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "corredor",
		SessionDate: time.Now(), SetNumber: 1, MediaURLs: media,
	}).Error)
}

func TestCalendarService_SessionInstanceDetail_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, 999999999, 1)
	require.Nil(t, resp)
	require.ErrorIs(t, err, ErrCalendarInstanceNotFound)
}

func TestCalendarService_SessionInstanceDetail_Forbidden(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "A")
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, f.instDay.ID, f.stranger.ID)
	require.Nil(t, resp)
	require.ErrorIs(t, err, ErrCalendarForbidden)

	resp, err = svc.SessionInstanceDetail(nil, f.instDay.ID, f.exMember.ID)
	require.Nil(t, resp)
	require.ErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_SessionInstanceDetail_DiaDelGrupo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "B")
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, f.instDay.ID, f.athlete.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, f.instDay.ID, resp.ID)
	assert.Equal(t, "Fartlek 5K", resp.Name)
	require.NotNil(t, resp.Description)
	assert.Equal(t, "Fartlek congelado", *resp.Description)
}

func TestCalendarService_SessionInstanceDetail_DiaDelEquipo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "C")
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, f.instDay.ID, resp.ID)
}

func TestCalendarService_SessionInstanceDetail_FeedbackHuerfana(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "D")
	svc := detailSvc(db)

	orphan := &dbs.SessionInstance{Name: "Huérfana con feedback"}
	require.NoError(t, db.Create(orphan).Error)
	exInst := &dbs.ExerciseInstance{Name: "Trote 400m", Kind: "training"}
	require.NoError(t, db.Create(exInst).Error)
	link := &dbs.SessionExerciseInstance{SessionInstanceID: orphan.ID, ExerciseInstanceID: exInst.ID, Role: "principal", RepeatCount: 4, RestMinutes: 2}
	require.NoError(t, db.Create(link).Error)
	teamID := f.team.ID
	detailFeedback(t, db, f.athlete.ID, orphan.ID, exInst.ID, &teamID)

	resp, err := svc.SessionInstanceDetail(nil, orphan.ID, f.athlete.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, orphan.ID, resp.ID)
	require.Len(t, resp.Exercises, 1)
	ex := resp.Exercises[0]
	assert.Equal(t, "Trote 400m", ex.Name)
	assert.Equal(t, "principal", ex.Role)
	assert.Equal(t, 4, ex.RepeatCount)
	assert.Equal(t, 2, ex.RestMinutes)
}

func TestCalendarService_SessionInstanceDetail_FeedbackAjeno(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "E")
	svc := detailSvc(db)

	orphan := &dbs.SessionInstance{Name: "Fb ajeno"}
	require.NoError(t, db.Create(orphan).Error)
	teamID := f.team.ID
	detailFeedback(t, db, f.athlete.ID, orphan.ID, 1, &teamID)

	resp, err := svc.SessionInstanceDetail(nil, orphan.ID, f.stranger.ID)
	require.Nil(t, resp)
	require.ErrorIs(t, err, ErrCalendarForbidden)
}

// presencialDetailFixture reutiliza la infraestructura del detalle pero con
// el día instanciado como training+presencial, y devuelve el día.
func presencialDetailFixture(t *testing.T, db *gorm.DB, tag string) (*detailFixture, *gorm.DB, *dbs.GroupCalendarDay) {
	t.Helper()
	f := detailFixtureSetup(t, db, tag)
	var day dbs.GroupCalendarDay
	require.NoError(t, db.Where("group_id = ?", f.group.ID).First(&day).Error)
	require.NoError(t, db.Model(&day).Updates(map[string]interface{}{"is_presencial": true}).Error)
	return f, db, &day
}

func TestCalendarService_SessionInstanceDetail_PresencialAbierta(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f, _, day := presencialDetailFixture(t, db, "PA")
	opened := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(day).Updates(map[string]interface{}{"presencial_opened_at": opened}).Error)
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, f.instDay.ID, f.athlete.ID)
	require.NoError(t, err)
	require.NotNil(t, resp.PresencialOpen)
	assert.True(t, *resp.PresencialOpen)
	require.NotNil(t, resp.PresencialOpenedAt)
	assert.Equal(t, opened.UTC(), resp.PresencialOpenedAt.UTC())
	assert.Nil(t, resp.PresencialClosedAt)
}

func TestCalendarService_SessionInstanceDetail_PresencialCerrada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f, _, day := presencialDetailFixture(t, db, "PC")
	opened := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	require.NoError(t, db.Model(day).Updates(map[string]interface{}{
		"presencial_opened_at": opened, "presencial_closed_at": closed,
	}).Error)
	svc := detailSvc(db)

	resp, err := svc.SessionInstanceDetail(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp.PresencialOpen)
	assert.False(t, *resp.PresencialOpen)
	require.NotNil(t, resp.PresencialClosedAt)
	assert.Equal(t, closed.UTC(), resp.PresencialClosedAt.UTC())
}

// Día NO presencial del fixture base: el detalle existe pero los 3 campos
// quedan fuera (ApplyPresencialState no los setea).
func TestCalendarService_SessionInstanceDetail_NoPresencial_SinCampo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := detailFixtureSetup(t, db, "F")

	resp, err := detailSvc(db).SessionInstanceDetail(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.PresencialOpen)
	assert.Nil(t, resp.PresencialOpenedAt)
	assert.Nil(t, resp.PresencialClosedAt)
}

// Los paths de calendario (que comparten el DTO) NO exponen los campos del
// estado presencial — Sprint D8: solo el detalle los setea.
func TestCalendarService_GetRange_NoExpondenEstadoPresencial(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f, db, _ := presencialDetailFixture(t, db, "GR")
	opened := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	svc := detailSvc(db)
	resp, err := svc.GetRange(nil, f.group.ID, f.owner.ID, opened.AddDate(0, 0, -1), opened.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, resp, 1)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "presencial_open")
	assert.NotContains(t, string(raw), "opened_at")
	assert.NotContains(t, string(raw), "closed_at")
}
