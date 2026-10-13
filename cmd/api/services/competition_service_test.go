package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// Tests del service de competencias (E4.1, D9) con DAO mockeado: guards,
// validaciones 400/403, transiciones y shape.

type mockCompetitionDAO struct {
	isOwnerFn          func(teamID, userID int64) (bool, error)
	isMemberFn         func(teamID, userID int64) (bool, error)
	countMembersFn     func(teamID int64, userIDs []int64) (int64, error)
	groupBelongsToFn   func(groupID, teamID int64) (bool, error)
	createFn           func(comp *dbs.Competition, participantIDs []int64) error
	findByIDFn         func(id int64) (*dbs.Competition, error)
	listParticipantsFn func(ids []int64) ([]dbs.CompetitionParticipant, error)
	listByTeamFn       func(teamID int64, status *string, groupID *int64) ([]dbs.Competition, error)
	isParticipantFn    func(competitionID, userID int64) (bool, error)
	approveFn          func(competitionID, approvedBy int64) (bool, error)
	rejectFn           func(competitionID int64, reason *string) (bool, error)
	finishFn           func(competitionID int64, finishedAt time.Time) (bool, error)
	deleteFn           func(competitionID int64) error
	hasUnseenFn        func(userID, teamID int64) (bool, error)
	aggregateWindowFn  func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error)

	createCalls, deleteCalls, approveCalls, rejectCalls, finishCalls int
}

func (m *mockCompetitionDAO) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	if m.isOwnerFn != nil {
		return m.isOwnerFn(teamID, userID)
	}
	return false, nil
}

func (m *mockCompetitionDAO) IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error) {
	if m.isMemberFn != nil {
		return m.isMemberFn(teamID, userID)
	}
	return false, nil
}

func (m *mockCompetitionDAO) CountTeamGroupMembers(ctx *gin.Context, teamID int64, userIDs []int64) (int64, error) {
	if m.countMembersFn != nil {
		return m.countMembersFn(teamID, userIDs)
	}
	return 0, nil
}

func (m *mockCompetitionDAO) GroupBelongsToTeam(ctx *gin.Context, groupID, teamID int64) (bool, error) {
	if m.groupBelongsToFn != nil {
		return m.groupBelongsToFn(groupID, teamID)
	}
	return false, nil
}

func (m *mockCompetitionDAO) Create(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error {
	m.createCalls++
	if m.createFn != nil {
		return m.createFn(comp, participantIDs)
	}
	return nil
}

func (m *mockCompetitionDAO) FindByID(ctx *gin.Context, id int64) (*dbs.Competition, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(id)
	}
	return nil, daos.ErrCompetitionNotFound
}

func (m *mockCompetitionDAO) ListParticipantsByCompetitionIDs(ctx *gin.Context, ids []int64) ([]dbs.CompetitionParticipant, error) {
	if m.listParticipantsFn != nil {
		return m.listParticipantsFn(ids)
	}
	return nil, nil
}

func (m *mockCompetitionDAO) ListWinnersByCompetitionIDs(ctx *gin.Context, ids []int64) ([]dbs.CompetitionWinner, error) {
	return nil, nil
}

func (m *mockCompetitionDAO) ListByTeam(ctx *gin.Context, teamID int64, status *string, groupID *int64) ([]dbs.Competition, error) {
	if m.listByTeamFn != nil {
		return m.listByTeamFn(teamID, status, groupID)
	}
	return nil, nil
}

func (m *mockCompetitionDAO) IsParticipant(ctx *gin.Context, competitionID, userID int64) (bool, error) {
	if m.isParticipantFn != nil {
		return m.isParticipantFn(competitionID, userID)
	}
	return false, nil
}

func (m *mockCompetitionDAO) Approve(ctx *gin.Context, competitionID, approvedBy int64) (bool, error) {
	m.approveCalls++
	if m.approveFn != nil {
		return m.approveFn(competitionID, approvedBy)
	}
	return true, nil
}

func (m *mockCompetitionDAO) Reject(ctx *gin.Context, competitionID int64, reason *string) (bool, error) {
	m.rejectCalls++
	if m.rejectFn != nil {
		return m.rejectFn(competitionID, reason)
	}
	return true, nil
}

func (m *mockCompetitionDAO) Finish(ctx *gin.Context, competitionID int64, finishedAt time.Time) (bool, error) {
	m.finishCalls++
	if m.finishFn != nil {
		return m.finishFn(competitionID, finishedAt)
	}
	return true, nil
}

func (m *mockCompetitionDAO) Delete(ctx *gin.Context, competitionID int64) error {
	m.deleteCalls++
	if m.deleteFn != nil {
		return m.deleteFn(competitionID)
	}
	return nil
}

func (m *mockCompetitionDAO) HasUnseenTeam(ctx *gin.Context, userID, teamID int64) (bool, error) {
	if m.hasUnseenFn != nil {
		return m.hasUnseenFn(userID, teamID)
	}
	return false, nil
}

func (m *mockCompetitionDAO) AggregateFeedbackWindow(ctx *gin.Context, teamID int64, participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
	if m.aggregateWindowFn != nil {
		return m.aggregateWindowFn(participantIDs, startAt, endAt)
	}
	return nil, nil
}

func newCompetitionServiceForTest(dao *mockCompetitionDAO) CompetitionServiceInterface {
	return NewCompetitionService(dao, nil, nil)
}

func validMetricCreateReq() competition.CreateCompetitionRequest {
	metric := "distance"
	end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	return competition.CreateCompetitionRequest{
		Type:           competition.TypeMetric,
		Metric:         &metric,
		StartAt:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndAt:          &end,
		ParticipantIDs: []int64{11, 12},
	}
}

func routeCreateReq() competition.CreateCompetitionRequest {
	radius := 50.0
	return competition.CreateCompetitionRequest{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           strPtr("{\"type\":\"LineString\",\"coordinates\":[[0,0]]}"),
		AcceptanceRadiusMeters: &radius,
		StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		ParticipantIDs:         []int64{11, 12},
	}
}

func fptr(v float64) *float64 { return &v }

func TestCompetitionService_CreateValidaciones(t *testing.T) {
	svc := newCompetitionServiceForTest(&mockCompetitionDAO{
		isOwnerFn:      func(teamID, userID int64) (bool, error) { return true, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) { return int64(len(userIDs)), nil },
	})

	cases := []struct {
		name string
		req  competition.CreateCompetitionRequest
	}{
		{"type inválido", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.Type = "otros"
			return req
		}()},
		{"metric sin metric", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.Metric = nil
			return req
		}()},
		{"metric fuera de whitelist", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			m := "jump"
			req.Metric = &m
			return req
		}()},
		{"end_at <= start_at", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.EndAt = &req.StartAt
			return req
		}()},
		{"metric sin end_at", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.EndAt = nil
			return req
		}()},
		{"metric con campos de route", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.RouteGeoJSON = strPtr("{}")
			return req
		}()},
		{"route sin geojson", competition.CreateCompetitionRequest{
			Type:                   competition.TypeRoute,
			StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			AcceptanceRadiusMeters: fptr(50.0),
			ParticipantIDs:         []int64{11, 12},
		}},
		{"route con radius <= 0", func() competition.CreateCompetitionRequest {
			req := routeCreateReq()
			req.AcceptanceRadiusMeters = fptr(0)
			return req
		}()},
		{"route con metric", func() competition.CreateCompetitionRequest {
			req := routeCreateReq()
			m := "distance"
			req.Metric = &m
			return req
		}()},
		{"1 solo participante", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.ParticipantIDs = []int64{11}
			return req
		}()},
		{"participantes con duplicados", func() competition.CreateCompetitionRequest {
			req := validMetricCreateReq()
			req.ParticipantIDs = []int64{11, 11}
			return req
		}()},
	}
	for _, tc := range cases {
		_, err := svc.Create(nil, 1, 10, tc.req)
		require.ErrorIs(t, err, ErrCompetitionInvalid, tc.name)
	}
}

func TestCompetitionService_GuardsDeCreate(t *testing.T) {
	// Caller fuera del team: 403 y DAO de escritura sin tocar.
	var calls int
	dao := &mockCompetitionDAO{
		isOwnerFn:  func(teamID, userID int64) (bool, error) { return false, nil },
		isMemberFn: func(teamID, userID int64) (bool, error) { calls++; return false, nil },
	}
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.Create(nil, 99, 10, validMetricCreateReq())
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Equal(t, 1, calls)
	assert.Zero(t, dao.createCalls)

	// Owner: status active, role trainer y sin poder incluyéndose.
	var capturedStatus, capturedRole string
	var capturedParticipants []int64
	ownerDao := func(fn func(comp *dbs.Competition, participantIDs []int64) error) *mockCompetitionDAO {
		return &mockCompetitionDAO{
			isOwnerFn: func(teamID, userID int64) (bool, error) { return userID == 1, nil },
			countMembersFn: func(teamID int64, userIDs []int64) (int64, error) {
				return int64(len(userIDs)), nil
			},
			createFn: fn,
		}
	}
	dao = ownerDao(func(comp *dbs.Competition, participantIDs []int64) error {
		capturedStatus = comp.Status
		capturedRole = comp.CreatedByRole
		capturedParticipants = participantIDs
		return nil
	})
	svc = newCompetitionServiceForTest(dao)

	// El trainer entre los participantes es 400.
	req := validMetricCreateReq()
	req.ParticipantIDs = []int64{1, 12}
	_, err = svc.Create(nil, 1, 10, req)
	require.ErrorIs(t, err, ErrCompetitionInvalid)
	assert.Zero(t, dao.createCalls)

	// Participante ajeno al team es 400.
	dao = ownerDao(func(comp *dbs.Competition, participantIDs []int64) error { return nil })
	dao.countMembersFn = func(teamID int64, userIDs []int64) (int64, error) {
		return 1, nil // solo 1 de 2 es miembro
	}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Create(nil, 1, 10, validMetricCreateReq())
	require.ErrorIs(t, err, ErrCompetitionInvalid)

	// group_id de otro team es 400.
	dao = ownerDao(func(comp *dbs.Competition, participantIDs []int64) error { return nil })
	dao.groupBelongsToFn = func(groupID, teamID int64) (bool, error) { return false, nil }
	svc = newCompetitionServiceForTest(dao)
	req = validMetricCreateReq()
	req.GroupID = ptrInt64(5)
	_, err = svc.Create(nil, 1, 10, req)
	require.ErrorIs(t, err, ErrCompetitionInvalid)

	// Owner válido.
	dao = ownerDao(func(comp *dbs.Competition, participantIDs []int64) error {
		capturedStatus = comp.Status
		capturedRole = comp.CreatedByRole
		capturedParticipants = participantIDs
		return nil
	})
	svc = newCompetitionServiceForTest(dao)
	resp, err := svc.Create(nil, 1, 10, validMetricCreateReq())
	require.NoError(t, err)
	assert.Equal(t, competition.StatusActive, capturedStatus)
	assert.Equal(t, competition.CreatorRoleTrainer, capturedRole)
	assert.Equal(t, []int64{11, 12}, capturedParticipants)
	assert.Equal(t, competition.StatusActive, resp.Status)

	// Corredor miembro: pending_approval + role runner.
	runnerDao := &mockCompetitionDAO{
		isOwnerFn:  func(teamID, userID int64) (bool, error) { return false, nil },
		isMemberFn: func(teamID, userID int64) (bool, error) { return true, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) {
			return int64(len(userIDs)), nil
		},
	}
	svc = newCompetitionServiceForTest(runnerDao)
	resp, err = svc.Create(nil, 77, 10, validMetricCreateReq())
	require.NoError(t, err)
	assert.Equal(t, competition.StatusPendingApproval, resp.Status)
	assert.Equal(t, competition.CreatorRoleRunner, resp.CreatedByRole)
}

func ptrInt64(v int64) *int64 { return &v }

func TestCompetitionService_Moderacion(t *testing.T) {
	moderationDao := func(status string) *mockCompetitionDAO {
		var dao *mockCompetitionDAO
		dao = &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				statusNow := status
				if dao.approveCalls > 0 {
					statusNow = competition.StatusActive
				}
				if dao.rejectCalls > 0 {
					statusNow = competition.StatusRejected
				}
				approvedBy := int64(1)
				if dao.approveCalls == 0 {
					approvedBy = 0
				}
				return &dbs.Competition{
					ID: id, TeamID: 10, Status: statusNow,
					CreatedByUserID: 77, CreatedByRole: competition.CreatorRoleRunner,
					ApprovedByUserID: func() *int64 {
						if approvedBy > 0 {
							return &approvedBy
						}
						return nil
					}(),
				}, nil
			},
		}
		return dao
	}

	// Caller fuera del team: 403 y sin tocar la transición.
	dao := moderationDao(competition.StatusPendingApproval)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return false, nil }
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.Approve(nil, 99, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Zero(t, dao.approveCalls)

	// Owner sobre pending: approve feliz.
	dao = moderationDao(competition.StatusPendingApproval)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	svc = newCompetitionServiceForTest(dao)
	resp, err := svc.Approve(nil, 1, 5)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusActive, resp.Status)
	assert.Equal(t, 1, dao.approveCalls)

	// Approve ya no es posible desde active.
	dao = moderationDao(competition.StatusActive)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	dao.approveFn = func(competitionID, approvedBy int64) (bool, error) { return false, nil }
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Approve(nil, 1, 5)
	require.ErrorIs(t, err, ErrCompetitionConflict)

	// Reject con reason persistida.
	dao = moderationDao(competition.StatusPendingApproval)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	var capturedReason *string
	dao.rejectFn = func(competitionID int64, reason *string) (bool, error) {
		capturedReason = reason
		return true, nil
	}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Reject(nil, 1, 5, strPtr("demasiado ambicioso"))
	require.NoError(t, err)
	require.NotNil(t, capturedReason)
	assert.Equal(t, "demasiado ambicioso", *capturedReason)

	// Reject sin reason: nil llega al DAO intacto.
	dao = moderationDao(competition.StatusPendingApproval)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	dao.rejectFn = func(competitionID int64, reason *string) (bool, error) {
		capturedReason = reason
		return true, nil
	}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Reject(nil, 1, 5, nil)
	require.NoError(t, err)
	assert.Nil(t, capturedReason)
}

func TestCompetitionService_Cancel(t *testing.T) {
	cancelDao := func(status string, creatorRole string) *mockCompetitionDAO {
		return &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{
					ID: id, TeamID: 10, Status: status,
					CreatedByUserID: 77, CreatedByRole: creatorRole,
				}, nil
			},
		}
	}

	// Creador corredor sobre pending: borra.
	dao := cancelDao(competition.StatusPendingApproval, competition.CreatorRoleRunner)
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.Cancel(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, dao.deleteCalls)

	// El owner NO puede cancelar la propuesta del corredor (spec).
	dao = cancelDao(competition.StatusPendingApproval, competition.CreatorRoleRunner)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Cancel(nil, 1, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Zero(t, dao.deleteCalls)

	// El corredor creador no puede cancelar algo que ya no está en pending.
	dao = cancelDao(competition.StatusActive, competition.CreatorRoleRunner)
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Cancel(nil, 77, 5)
	require.ErrorIs(t, err, ErrCompetitionConflict)
	assert.Zero(t, dao.deleteCalls)

	// El trainer creador tampoco: la cancelación es del corredor.
	dao = cancelDao(competition.StatusPendingApproval, competition.CreatorRoleTrainer)
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Cancel(nil, 77, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)

	// Competencia ajena.
	dao = cancelDao(competition.StatusPendingApproval, competition.CreatorRoleRunner)
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Cancel(nil, 42, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
}

func TestCompetitionService_Listado(t *testing.T) {
	calls := 0
	dao := &mockCompetitionDAO{
		isMemberFn: func(teamID, userID int64) (bool, error) { calls++; return false, nil },
	}
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.ListByTeam(nil, 99, 10, "", nil)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Equal(t, 1, calls, "un solo guard al fallar")

	// Status fuera de whitelist → 400 sin tocar el DAO de listado.
	dao = &mockCompetitionDAO{}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.ListByTeam(nil, 99, 10, "invalido", nil)
	require.ErrorIs(t, err, ErrCompetitionInvalid)

	// Feliz con filtros y unseen embebido.
	var capturedStatus *string
	dao = &mockCompetitionDAO{
		isMemberFn:  func(teamID, userID int64) (bool, error) { return true, nil },
		hasUnseenFn: func(userID, teamID int64) (bool, error) { return true, nil },
		listByTeamFn: func(teamID int64, status *string, groupID *int64) ([]dbs.Competition, error) {
			capturedStatus = status
			metric := "distance"
			return []dbs.Competition{{
				ID: 5, TeamID: 10, Type: competition.TypeMetric, Metric: &metric,
				Status:  competition.StatusActive,
				StartAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			}}, nil
		},
	}
	svc = newCompetitionServiceForTest(dao)
	resp, err := svc.ListByTeam(nil, 3, 10, competition.StatusActive, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedStatus)
	assert.Equal(t, competition.StatusActive, *capturedStatus)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "distance", *resp.Data[0].Metric)
	assert.Equal(t, []int64{}, resp.Data[0].Participants, "batch vacío de participants = slice vacío")
	assert.Equal(t, []int64{}, resp.Data[0].WinnerUserIDs)
	assert.True(t, resp.HasUnseen)
	assert.Empty(t, resp.Data[0].Progress, "sin progress en el listado")
}

func TestCompetitionService_GetDetalleProgreso(t *testing.T) {
	// Reloj fijo para la ventana [start_at, end_at o now].
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	gametime.Now = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, gametime.Location) }

	comp := &dbs.Competition{
		ID: 5, TeamID: 10, Type: competition.TypeMetric,
		Metric:  strPtr("distance"),
		StartAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Status:  competition.StatusActive,
	}
	dao := &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) { return comp, nil },
		isOwnerFn:  func(teamID, userID int64) (bool, error) { return userID == 2, nil },
		isMemberFn: func(teamID, userID int64) (bool, error) { return userID == 3, nil },
		listParticipantsFn: func(ids []int64) ([]dbs.CompetitionParticipant, error) {
			return []dbs.CompetitionParticipant{
				{CompetitionID: 5, UserID: 11},
				{CompetitionID: 5, UserID: 12},
			}, nil
		},
		aggregateWindowFn: func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
			return []dbs.CompetitionMetricAgg{
				{UserID: 12, SumDistance: fptr(7000)},
				{UserID: 11, SumDistance: fptr(3000)},
			}, nil
		},
	}
	svc := newCompetitionServiceForTest(dao)

	// Caller fuera del team: 403.
	_, err := svc.Get(nil, 99, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)

	// Miembro del team: 200 con progreso por participante.
	resp, err := svc.Get(nil, 3, 5)
	require.NoError(t, err)
	require.Equal(t, competition.StatusActive, resp.Data.Status)
	require.Len(t, resp.Data.Progress, 2)
	assert.Equal(t, int64(11), resp.Data.Progress[0].UserID)
	require.NotNil(t, resp.Data.Progress[0].Value)
	assert.InDelta(t, 3000.0, *resp.Data.Progress[0].Value, 0.0001)
	assert.InDelta(t, 7000.0, *resp.Data.Progress[1].Value, 0.0001)

	// type=route queda sin progress (E4.2).
	comp.Type = competition.TypeRoute
	comp.Metric = nil
	dao.aggregateWindowFn = func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
		t.Fatal("la ruta no debe pedir agregación hasta el matching de E4.2")
		return nil, nil
	}
	resp, err = svc.Get(nil, 3, 5)
	require.NoError(t, err)
	assert.Empty(t, resp.Data.Progress)
}

func TestCompetitionService_Close(t *testing.T) {
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	gametime.Now = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, gametime.Location) }

	closeDao := func(status string, endAt *time.Time, createdBy int64, role string) *mockCompetitionDAO {
		finishedAt := time.Date(2026, 10, 20, 12, 0, 0, 0, gametime.Location)
		var closed bool
		return &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				statusNow := status
				if closed {
					statusNow = competition.StatusFinished
				}
				return &dbs.Competition{
					ID: id, TeamID: 10, Type: competition.TypeMetric, Status: statusNow,
					StartAt:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
					EndAt:           endAt,
					CreatedByUserID: createdBy, CreatedByRole: role,
					FinishedAt: func() *time.Time {
						if closed {
							return &finishedAt
						}
						return nil
					}(),
				}, nil
			},
			isMemberFn: func(teamID, userID int64) (bool, error) { return userID == createdBy, nil },
			finishFn: func(competitionID int64, at time.Time) (bool, error) {
				closed = true
				return true, nil
			},
		}
	}
	endPassed := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	endFuture := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)

	// Caller que no es creador ni owner: 403.
	dao := closeDao(competition.StatusActive, &endPassed, 77, competition.CreatorRoleRunner)
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.Close(nil, 99, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Zero(t, dao.finishCalls)

	// Corredor creador cierra (ventana terminada): 200.
	resp, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusFinished, resp.Status)
	assert.Equal(t, 1, dao.finishCalls)

	// Cierre antes de end_at: 400 y sin persistir cierre.
	dao = closeDao(competition.StatusActive, &endFuture, 77, competition.CreatorRoleRunner)
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Close(nil, 77, 5)
	require.ErrorContains(t, err, "competencia no puede cerrarse todavía")
	assert.Zero(t, dao.finishCalls)

	// Owner cierra la del corredor.
	dao = closeDao(competition.StatusActive, &endPassed, 77, competition.CreatorRoleRunner)
	dao.isOwnerFn = func(teamID, userID int64) (bool, error) { return true, nil }
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Close(nil, 1, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, dao.finishCalls)

	// Cerrar algo no activo: 400.
	dao = closeDao(competition.StatusPendingApproval, nil, 77, competition.CreatorRoleRunner)
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.Close(nil, 77, 5)
	require.ErrorIs(t, err, ErrCompetitionConflict)
	assert.Zero(t, dao.finishCalls)
}

func TestCompetitionsMetricValueFromAgg(t *testing.T) {
	assert.Nil(t, metricValueFromAgg(dbs.CompetitionMetricAgg{}, nil))
	assert.Nil(t, metricValueFromAgg(dbs.CompetitionMetricAgg{}, strPtr("distance")))
	assert.InDelta(t, 5000.0, *metricValueFromAgg(
		dbs.CompetitionMetricAgg{SumDistance: fptr(5000)}, strPtr("distance")), 0.0001)
	assert.InDelta(t, 900000.0, *metricValueFromAgg(
		dbs.CompetitionMetricAgg{SumDurationMs: ptrInt64Ms(900000)}, strPtr("duration")), 0.0001)
	assert.Nil(t, metricValueFromAgg(dbs.CompetitionMetricAgg{}, strPtr("duration")))
	// pace acumulado = razón.
	assert.InDelta(t, 300.0, *metricValueFromAgg(
		dbs.CompetitionMetricAgg{SumDurationMs: ptrInt64Ms(1500000), SumDistance: fptr(5000)},
		strPtr("pace")), 0.0001)
	assert.InDelta(t, 18.0, *metricValueFromAgg(
		dbs.CompetitionMetricAgg{MaxPeak: fptr(18)}, strPtr("peak_speed")), 0.0001)
	assert.Nil(t, metricValueFromAgg(
		dbs.CompetitionMetricAgg{SumDurationMs: ptrInt64Ms(100), SumDistance: fptr(0)},
		strPtr("pace")), "pace con distance 0 = null")
}

func ptrInt64Ms(v int64) *int64 { return &v }

func TestCompetitionService_ModeracionTrainerCreador(t *testing.T) {
	dao := &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			return &dbs.Competition{
				ID: id, TeamID: 10, Status: competition.StatusPendingApproval,
				CreatedByUserID: 77, CreatedByRole: competition.CreatorRoleTrainer,
			}, nil
		},
	}
	svc := newCompetitionServiceForTest(dao)
	// La moderación sí corre (el guard admite al trainer creador), aunque el
	// Get final falla por el guard mockeado sin memberships.
	_, err := svc.Approve(nil, 77, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Equal(t, 1, dao.approveCalls)
}

func TestCompetitionService_ModeracionPushAlCreador(t *testing.T) {
	var dao *mockCompetitionDAO
	dao = &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			statusNow := competition.StatusPendingApproval
			if dao.approveCalls > 0 {
				statusNow = competition.StatusActive
			}
			return &dbs.Competition{
				ID: id, TeamID: 10, Status: statusNow,
				CreatedByUserID: 77, CreatedByRole: competition.CreatorRoleRunner,
				ApprovedByUserID: func() *int64 { if dao.approveCalls > 0 { i := int64(1); return &i }; return nil }(),
			}, nil
		},
		isOwnerFn: func(teamID, userID int64) (bool, error) { return userID == 1, nil },
	}
	pushDAO := mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		return []dbs.PushToken{{Token: "ExponentPushToken[test]"}}, nil
	}}
	client := &mockExpoPushClient{}
	svc := NewCompetitionService(dao, &pushDAO, client)

	resp, err := svc.Approve(nil, 1, 5)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusActive, resp.Status)
	assert.Equal(t, 1, client.sendCallCount)
	assert.Equal(t, "/teams/10", client.lastData["route"])
}
