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

// presencialTracker acumula las lecturas combinadas día+owner y los writes
// para afirmar no-ops y cantidad de round trips.
type presencialTracker struct {
	dayLookups int
	setCalls   []string
}

func (pt *presencialTracker) noWrites() bool {
	return len(pt.setCalls) == 0
}

func presencialSvcWith(day *dbs.GroupCalendarDay, ownerID int64) (*presencialSessionService, *dbs.GroupCalendarDay, *presencialTracker) {
	track := &presencialTracker{}
	dayDao := &mockGroupCalendarDao{findWithTeamOwnerFn: func(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, *int64, error) {
		track.dayLookups++
		if day == nil {
			return nil, nil, nil
		}
		owner := ownerID
		return day, &owner, nil
	}}
	dayDao.setPresencialOpenedAtFn = func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
		track.setCalls = append(track.setCalls, "opened")
		day.PresencialOpenedAt = &at
		return day, true, nil
	}
	dayDao.setPresencialClosedAtFn = func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
		track.setCalls = append(track.setCalls, "closed")
		day.PresencialClosedAt = &at
		return day, true, nil
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}
	return svc, day, track
}

func TestPresencial_CheckAthleteEntry_NoDay_NoGate(t *testing.T) {
	// Instancia sin día (huérfana): sin gate. Una sola lectura combinada.
	svc, _, track := presencialSvcWith(nil, 7)

	day, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
	assert.Nil(t, day)
	assert.Equal(t, 1, track.dayLookups)
	assert.True(t, track.noWrites())
}

func TestPresencial_CheckAthleteEntry_NonPresencial_NoGate(t *testing.T) {
	rest := fixturePresencialDay(5, nil, nil)
	rest.IsPresencial = false
	svc, _, track := presencialSvcWith(rest, 0)

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
	assert.Equal(t, 1, track.dayLookups)
	assert.True(t, track.noWrites())
}

func TestPresencial_CheckAthleteEntry_OwnerExempt(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, _, err := svc.CheckAthleteEntry(nil, 42, 7)

	require.NoError(t, err)
	require.NotNil(t, day, "el día cargado por el gate sale por el passthrough")
	assert.True(t, track.noWrites())
}

func TestPresencial_CheckAthleteEntry_NotOpen(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, _ := presencialSvcWith(own, 7)

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, ErrRunnerSessionNotOpen)
}

func TestPresencial_CheckAthleteEntry_ClosedWins(t *testing.T) {
	opened := time.Now()
	closed := opened.Add(time.Hour)
	own := fixturePresencialDay(5, &opened, &closed)
	svc, _, _ := presencialSvcWith(own, 7)

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, ErrRunnerSessionClosed)
}

func TestPresencial_CheckAthleteEntry_OpenAndRunning_OK(t *testing.T) {
	opened := time.Now()
	own := fixturePresencialDay(5, &opened, nil)
	svc, _, _ := presencialSvcWith(own, 7)

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.NoError(t, err)
}

func TestPresencial_CheckAthleteEntry_BothLookupsFail_PropagatesDayReadError(t *testing.T) {
	combinedErr := errors.New("combined boom")
	dayErr := errors.New("day boom")
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			return nil, nil, combinedErr
		},
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			return nil, dayErr
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, dayErr, "si ambos lookups fallan se conserva el error del día")
}

// Fallback de paridad: el owner lookup combinado falla pero el día se
// recupera por la lectura sola → el owner queda ausente y el gate sigue su
// curso (409 de sesión sin abrir), no un error de infraestructura.
func TestPresencial_CheckAthleteEntry_CombinedLookupError_DayReadSucceeds(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			return nil, nil, errors.New("combined boom")
		},
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			return own, nil
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	_, _, err := svc.CheckAthleteEntry(nil, 42, 99)

	require.ErrorIs(t, err, ErrRunnerSessionNotOpen)
}

// El gate no encadena lecturas de group/team: una sola consulta resuelve
// día + owner.
func TestPresencial_CheckAthleteEntry_UnaSolaLecturaDiaYOwner(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	combined := 0
	single := 0
	dayDao := &mockGroupCalendarDao{
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			single++
			return own, nil
		},
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			combined++
			owner := int64(7)
			return own, &owner, nil
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	_, _, err := svc.CheckAthleteEntry(nil, 42, 7)

	require.NoError(t, err)
	assert.Equal(t, 1, combined)
	assert.Zero(t, single, "no se usa el lookup de día separado")
}

func TestPresencial_OnRunnerCreated_OwnerOpens(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil, nil)

	require.NoError(t, err)
	assert.True(t, mutated)
	require.NotNil(t, day)
	require.NotNil(t, day.PresencialOpenedAt)
	assert.WithinDuration(t, time.Now(), *day.PresencialOpenedAt, 2*time.Second)
	assert.Equal(t, int64(5), day.ID)
	assert.Equal(t, []string{"opened"}, track.setCalls)
}

// El hook reutiliza día y owner resueltos por el gate (D3): no repite la
// lectura combinada.
func TestPresencial_OnRunnerCreated_GateDayAndOwner_SinLecturaCombinada(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)
	owner := int64(7)

	_, mutated, err := svc.OnRunnerCreated(nil, 42, 7, own, &owner)

	require.NoError(t, err)
	assert.True(t, mutated)
	assert.Equal(t, []string{"opened"}, track.setCalls)
	assert.Zero(t, track.dayLookups, "día y owner vienen del gate")

	// Sin mutación (no-owner) no hay writes ni lecturas extra.
	svcNotOwner, _, trackNotOwner := presencialSvcWith(own, 7)

	_, mutated, err = svcNotOwner.OnRunnerCreated(nil, 42, 99, own, &owner)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.True(t, trackNotOwner.noWrites())
	assert.Zero(t, trackNotOwner.dayLookups)
}

// Fallback seguro del hook: la lectura combinada falla pero la lectura de día
// sola funciona → owner ausente y no-op exitoso (no 500 después del write).
func TestPresencial_OnRunnerCreated_CombinedLookupError_DayReadSucceeds_Noop(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	combined, dayOnly := 0, 0
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			combined++
			return nil, nil, errors.New("combined boom")
		},
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			dayOnly++
			return own, nil
		},
		setPresencialOpenedAtFn: func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
			t.Fatal("un owner ausente no debe escribir")
			return nil, false, nil
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil, nil)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.Equal(t, 1, combined)
	assert.Equal(t, 1, dayOnly)
}

// Si el fallback tampoco puede leer el día, se propaga ese error (paridad con
// la lectura de día previa).
func TestPresencial_OnRunnerCreated_BothLookupsFail_PropagatesDayReadError(t *testing.T) {
	dayErr := errors.New("day boom")
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			return nil, nil, errors.New("combined boom")
		},
		findBySessionInstanceIDFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, error) {
			return nil, dayErr
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	_, _, err := svc.OnRunnerCreated(nil, 42, 7, nil, nil)

	require.ErrorIs(t, err, dayErr)
}

func TestPresencial_OnRunnerCreated_NotOwner_Noop(t *testing.T) {
	own := fixturePresencialDay(5, nil, nil)
	svc, _, track := presencialSvcWith(own, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 99, nil, nil)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.True(t, track.noWrites())
}

func TestPresencial_OnRunnerCreated_NonPresencial_Noop(t *testing.T) {
	rest := fixturePresencialDay(5, nil, nil)
	rest.IsPresencial = false
	svc, _, track := presencialSvcWith(rest, 7)

	day, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil, nil)

	require.NoError(t, err)
	assert.False(t, mutated)
	assert.Nil(t, day)
	assert.True(t, track.noWrites())
}

func TestPresencial_OnRunnerCreated_AlreadyOpen_NoMutation(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	own := fixturePresencialDay(5, &opened, nil)
	// Set fn devuelve nil,false: simula el guard SQL sin mutación.
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			owner := int64(7)
			return own, &owner, nil
		},
		setPresencialOpenedAtFn: func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
			return nil, false, nil
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

	got, mutated, err := svc.OnRunnerCreated(nil, 42, 7, nil, nil)

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
	assert.WithinDuration(t, time.Now(), *day.PresencialClosedAt, 2*time.Second)
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
	assert.True(t, track.noWrites())
}

func TestPresencial_OnRunnerFinished_AlreadyClosed_NoMutation(t *testing.T) {
	opened := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(time.Hour)
	own := fixturePresencialDay(5, &opened, &closed)
	dayDao := &mockGroupCalendarDao{
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			owner := int64(7)
			return own, &owner, nil
		},
		setPresencialClosedAtFn: func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
			return nil, false, nil
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

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
		findWithTeamOwnerFn: func(ctx *gin.Context, id int64) (*dbs.GroupCalendarDay, *int64, error) {
			owner := int64(7)
			return own, &owner, nil
		},
		setPresencialClosedAtFn: func(ctx *gin.Context, dayID int64, at time.Time) (*dbs.GroupCalendarDay, bool, error) {
			return nil, false, boom
		},
	}
	svc := &presencialSessionService{calendarDayDao: dayDao}

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

	gateway := &presencialSessionService{calendarDayDao: dao}
	_, _, err := gateway.CheckAthleteEntry(nil, f.instDay.ID, f.athlete.ID)
	require.ErrorIs(t, err, ErrRunnerSessionNotOpen, "re-activada y sin abrir: el gate rechaza")
	_, _, err = gateway.CheckAthleteEntry(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err, "el owner sigue exento")

	resp, err := detailSvc(db).SessionInstanceDetail(nil, f.instDay.ID, f.owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp.PresencialOpen)
	assert.False(t, *resp.PresencialOpen)
	assert.Nil(t, resp.PresencialOpenedAt)
	assert.Nil(t, resp.PresencialClosedAt)
}
