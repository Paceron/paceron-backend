package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// fixturePresencialDay arma un día training+presencial opcionalmente
// abierto/cerrado, para los tests del gateway presencial.
func fixturePresencialDay(dayID int64, opened, closed *time.Time) *dbs.GroupCalendarDay {
	return &dbs.GroupCalendarDay{
		ID:                 dayID,
		GroupID:            3,
		Kind:               "training",
		SessionInstanceID:  int64Ptr(42),
		IsPresencial:       true,
		PresencialOpenedAt: opened,
		PresencialClosedAt: closed,
	}
}

// presencialTracker acumula los efectos observados del gateway (owner lookups
// y writes) para afirmar no-op completos.
type presencialTracker struct {
	ownerLookups int
	setCalls     []string
}

func (pt *presencialTracker) noop() bool {
	return pt.ownerLookups == 0 && len(pt.setCalls) == 0
}

func presencialSvcWith(day *dbs.GroupCalendarDay, ownerOfGroup int64) (*presencialSessionService, *dbs.GroupCalendarDay, *presencialTracker) {
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error) {
		return day, nil
	}}
	track := &presencialTracker{}
	dayDao.setPresencialOpenedAtFn = func(ctx *gin.Context, dayID int64, at time.Time) (bool, error) {
		track.setCalls = append(track.setCalls, "opened")
		day.PresencialOpenedAt = &at
		return true, nil
	}
	dayDao.setPresencialClosedAtFn = func(ctx *gin.Context, dayID int64, at time.Time) (bool, error) {
		track.setCalls = append(track.setCalls, "closed")
		day.PresencialClosedAt = &at
		return true, nil
	}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		track.ownerLookups++
		return &dbs.Group{ID: id, TeamID: 77}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: ownerOfGroup}, nil
	}}
	svc := &presencialSessionService{calendarDayDao: dayDao, groupDao: groupDao, teamDao: teamDao}
	return svc, day, track
}

func TestPresencial_CheckAthleteEntry_NoDay_NoGate(t *testing.T) {
	// Instancia sin día (huérfana): sin gate y sin lookup de owner.
	svc, _, track := presencialSvcWith(nil, 7)

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
	assert.True(t, track.noop())
}

func TestPresencial_CheckAthleteEntry_NonPresencial_NoGate(t *testing.T) {
	rest := fixturePresencialDay(5, nil, nil)
	rest.IsPresencial = false
	svc, _, track := presencialSvcWith(rest, 0)

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
	assert.True(t, track.noop())
}

func TestPresencial_CheckAthleteEntry_OwnerExempt(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, err := svc.CheckAthleteEntry(nil, 42, 7)

	require.NoError(t, err)
	require.NotNil(t, day, "el día cargado por el gate sale por el passthrough")
	assert.Empty(t, track.setCalls)
}

func TestPresencial_CheckAthleteEntry_NotOpen(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, _ := presencialSvcWith(own, 7)

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, ErrRunnerSessionNotOpen)
}

func TestPresencial_CheckAthleteEntry_ClosedWins(t *testing.T) {
	opened := time.Now()
	closed := opened.Add(time.Hour)
	own := fixturePresencialDay(5, &opened, &closed)
	svc, _, _ := presencialSvcWith(own, 7)

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, ErrRunnerSessionClosed)
}

func TestPresencial_CheckAthleteEntry_OpenAndRunning_OK(t *testing.T) {
	opened := time.Now()
	own := fixturePresencialDay(5, &opened, nil)
	svc, _, _ := presencialSvcWith(own, 7)

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
}

func TestPresencial_CheckAthleteEntry_DayLookupError(t *testing.T) {
	svc, _, _ := presencialSvcWith(nil, 7)
	boom := errors.New("boom")
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
		return nil, boom
	}}
	svc.calendarDayDao = dayDao

	_, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, boom)
}

func TestPresencial_OnRunnerCreated_OwnerOpens(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil)

	require.NoError(t, err)
	assert.True(t, mutated)
	require.NotNil(t, day)
	require.NotNil(t, day.PresencialOpenedAt)
	assert.Equal(t, int64(5), day.ID)
	assert.Equal(t, []string{"opened"}, track.setCalls)
}

// El día que trae el gate (mismo POST, FK 1:1) se consume sin re-consulta:
// la única lectura extra es el re-read post-write del evento.
func TestPresencial_OnRunnerCreated_GateDay_SinReconsulta(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)
	dayLookups := 0
	svc.calendarDayDao.(*mockGroupCalendarDao).findBySessionInstanceIDFn = func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
		dayLookups++
		return own, nil
	}

	_, mutated, err := svc.OnRunnerCreated(nil, 42, 7, own)

	require.NoError(t, err)
	assert.True(t, mutated)
	assert.Equal(t, []string{"opened"}, track.setCalls)
	assert.Equal(t, 1, dayLookups, "solo el re-read post-write")

	// Sin mutación (no-owner) no hay ninguna lectura del día.
	svcNotOwner, _, trackNotOwner := presencialSvcWith(own, 7)
	lookupsNotOwner := 0
	svcNotOwner.calendarDayDao.(*mockGroupCalendarDao).findBySessionInstanceIDFn = func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
		lookupsNotOwner++
		return own, nil
	}

	_, mutated, err = svcNotOwner.OnRunnerCreated(nil, 42, 99, own)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Empty(t, trackNotOwner.setCalls)
	assert.Zero(t, lookupsNotOwner)
}

func TestPresencial_OnRunnerCreated_NotOwner_Noop(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 99, nil)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.Empty(t, track.setCalls)
}

func TestPresencial_OnRunnerCreated_NonPresencial_Noop(t *testing.T) {
	rest := fixturePresencialDay(5, nil, nil)
	rest.IsPresencial = false
	svc, _, track := presencialSvcWith(rest, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.True(t, track.noop())
}

func TestPresencial_OnRunnerCreated_AlreadyOpen_NoMutation(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	own := fixturePresencialDay(5, &opened, nil)
	// Sin fn en setPresencialOpenedAt → default (false, no write): simula el
	// guard SQL sin mutación.
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
		return own, nil
	}}
	svc := &presencialSessionService{calendarDayDao: dayDao,
		groupDao: &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 77}, nil
		}},
		teamDao: &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, OwnerID: 7}, nil
		}},
	}

	got, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil)

	require.NoError(t, err)
	assert.False(t, mutated, "re-Play del owner no vuelve a abrir")
	assert.Nil(t, got)
}

func TestPresencial_OnRunnerFinished_OwnerCloses(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	own := fixturePresencialDay(5, &opened, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerFinished(nil, 42, 7)

	require.NoError(t, err)
	assert.True(t, mutated)
	require.NotNil(t, day)
	require.NotNil(t, day.PresencialClosedAt)
	assert.Equal(t, []string{"closed"}, track.setCalls)
}

func TestPresencial_OnRunnerFinished_NotOwner_Noop(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	own := fixturePresencialDay(5, &opened, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerFinished(nil, 42, 99)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.Empty(t, track.setCalls)
}

func TestPresencial_OnRunnerFinished_AlreadyClosed_NoMutation(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(time.Hour)
	own := fixturePresencialDay(5, &opened, &closed)
	// Sin fn en setPresencialClosedAt → default (false, no write): guarda SQL.
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
		return own, nil
	}}
	svc := &presencialSessionService{calendarDayDao: dayDao,
		groupDao: &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 77}, nil
		}},
		teamDao: &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, OwnerID: 7}, nil
		}},
	}

	got, mutated, err := svc.OnRunnerFinished(nil, 42, 7)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, got)
}

func TestPresencial_SetWriteError_Surfaces(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	own := fixturePresencialDay(5, &opened, nil)
	boom := errors.New("boom")
	dayDao := &mockGroupCalendarDao{
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			return own, nil
		},
		setPresencialClosedAtFn: func(ctx *gin.Context, dayID int64, at time.Time) (bool, error) { return false, boom },
	}
	svc := &presencialSessionService{calendarDayDao: dayDao,
		groupDao: &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 77}, nil
		}},
		teamDao: &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, OwnerID: 7}, nil
		}},
	}

	day, mutated, err := svc.OnRunnerFinished(nil, 42, 7)

	require.ErrorIs(t, err, boom)
	assert.False(t, mutated)
	assert.Nil(t, day)
}

// D5 end-to-end: apagar is_presencial limpia opened/closed; re-activar
// presencial arranca sin apertura — el gate devuelve session_not_open y el
// detalle muestra presencial_open=false con timestamps NULL.
func TestPresencial_ReactivarPresencial_ArrancaSinEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f, _, day := presencialDetailFixture(t, db, "RP")
	opened := time.Date(2027, 3, 10, 17, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(day).Updates(map[string]interface{}{"presencial_opened_at": opened}).Error)
	dao := daos.NewGroupCalendarDayDao(db)

	off := &dbs.GroupCalendarDay{GroupID: day.GroupID, Date: day.Date, Kind: day.Kind, SessionInstanceID: day.SessionInstanceID}
	require.NoError(t, dao.Upsert(nil, off))
	on := &dbs.GroupCalendarDay{GroupID: day.GroupID, Date: day.Date, Kind: day.Kind, SessionInstanceID: day.SessionInstanceID, IsPresencial: true}
	require.NoError(t, dao.Upsert(nil, on))

	gateway := &presencialSessionService{calendarDayDao: dao, groupDao: daos.NewGroupDao(db), teamDao: daos.NewTeamDao(db)}
	_, err := gateway.CheckAthleteEntry(nil, f.instDay.ID, f.athlete.ID)
	require.ErrorIs(t, err, ErrRunnerSessionNotOpen, "re-activada y sin abrir: el gate rechaza")
	_, err = gateway.CheckAthleteEntry(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err, "el owner sigue exento")

	resp, err := detailSvc(db).SessionInstanceDetail(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp.PresencialOpen)
	assert.False(t, *resp.PresencialOpen)
	assert.Nil(t, resp.PresencialOpenedAt)
	assert.Nil(t, resp.PresencialClosedAt)
}
