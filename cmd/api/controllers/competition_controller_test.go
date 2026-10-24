package controllers

// Competencias (E4.1): guards/400/404 + no-leak del controller.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gorilla/websocket"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

type mockCompetitionService struct {
	createFn      func(callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error)
	approveFn     func(callerID, competitionID int64) (*competition.CompetitionResponse, error)
	rejectFn      func(callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error)
	cancelFn      func(callerID, competitionID int64) (*competition.CompetitionResponse, error)
	listFn        func(callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error)
	getFn         func(callerID, competitionID int64) (*competition.CompetitionDetailResponse, error)
	listAttempts  func(callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error)
	closeFn       func(callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error)
	createAttempt func(callerID, competitionID int64) (*dbs.RunnerSession, error)

	createCalls, approveCalls, rejectCalls, cancelCalls, listCalls, getCalls, closeCalls, attemptCalls int
	listAttemptsCalls                                                                                  int
	lastAthleteUserID                                                                                  int64
	lastReason                                                                                         *string
	lastStatus                                                                                         string
	lastGroupID                                                                                        *int64
}

func (m *mockCompetitionService) Create(ctx *gin.Context, callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error) {
	m.createCalls++
	if m.createFn != nil {
		return m.createFn(callerID, teamID, req)
	}
	return &competition.CompetitionResponse{ID: 5, Status: competition.StatusActive}, nil
}

func (m *mockCompetitionService) Approve(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	m.approveCalls++
	if m.approveFn != nil {
		return m.approveFn(callerID, competitionID)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusActive}, nil
}

func (m *mockCompetitionService) Reject(ctx *gin.Context, callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error) {
	m.rejectCalls++
	m.lastReason = reason
	if m.rejectFn != nil {
		return m.rejectFn(callerID, competitionID, reason)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusRejected}, nil
}

func (m *mockCompetitionService) Cancel(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	m.cancelCalls++
	if m.cancelFn != nil {
		return m.cancelFn(callerID, competitionID)
	}
	return nil, nil
}

func (m *mockCompetitionService) ListByTeam(ctx *gin.Context, callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error) {
	m.listCalls++
	m.lastStatus = status
	m.lastGroupID = groupID
	if m.listFn != nil {
		return m.listFn(callerID, teamID, status, groupID)
	}
	return &competition.CompetitionsListResponse{Data: []competition.CompetitionResponse{}}, nil
}

func (m *mockCompetitionService) Get(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionDetailResponse, error) {
	m.getCalls++
	if m.getFn != nil {
		return m.getFn(callerID, competitionID)
	}
	return &competition.CompetitionDetailResponse{Data: competition.CompetitionResponse{ID: competitionID}}, nil
}

func (m *mockCompetitionService) ListAttempts(ctx *gin.Context, callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error) {
	m.listAttemptsCalls++
	m.lastAthleteUserID = athleteUserID
	if m.listAttempts != nil {
		return m.listAttempts(callerID, competitionID, athleteUserID)
	}
	return &competition.CompetitionAttemptsResponse{Data: []competition.CompetitionAttemptItem{}}, nil
}

func (m *mockCompetitionService) Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error) {
	m.closeCalls++
	if m.closeFn != nil {
		return m.closeFn(callerID, competitionID)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusFinished}, nil, nil
}

func (m *mockCompetitionService) CreateAttempt(ctx *gin.Context, callerID, competitionID int64) (*dbs.RunnerSession, error) {
	m.attemptCalls++
	if m.createAttempt != nil {
		return m.createAttempt(callerID, competitionID)
	}
	instanceID := int64(42)
	return &dbs.RunnerSession{
		ID: 77, SessionInstanceID: instanceID, AthleteUserID: callerID,
		Status: "wip", CompetitionID: &competitionID,
	}, nil
}

func setupCompetitionRouterS(t *testing.T, svc services.CompetitionServiceInterface, authUserID int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	cc := NewCompetitionController(svc, nil)
	router.POST("/api/v1/teams/:id/competitions", cc.Create)
	router.GET("/api/v1/teams/:id/competitions", cc.ListByTeam)
	router.PATCH("/api/v1/competitions/:id/approve", cc.Approve)
	router.PATCH("/api/v1/competitions/:id/reject", cc.Reject)
	router.PATCH("/api/v1/competitions/:id/close", cc.Close)
	router.DELETE("/api/v1/competitions/:id/cancel", cc.Cancel)
	router.GET("/api/v1/competitions/:id", cc.GetDetail)
	router.GET("/api/v1/competitions/:id/attempts", cc.ListAttempts)
	router.POST("/api/v1/competitions/:id/attempts", cc.CreateAttempt)
	return router
}

func doCompetitionRequest(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func competitionErrorMessage(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	msg, ok := out["message"].(string)
	require.True(t, ok)
	return msg
}

func TestCompetitionController_Unauthorized(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 0)
	assert.Equal(t, http.StatusUnauthorized, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "").Code)
	assert.Equal(t, http.StatusUnauthorized, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/close", "").Code)
}

func TestCompetitionController_PathInvalido_DAONoTocado(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	require.Equal(t, http.StatusBadRequest, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/abc/competitions", "").Code)
	require.Equal(t, http.StatusBadRequest, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/-1/approve", "").Code)
	assert.Zero(t, svc.approveCalls)
	assert.Zero(t, svc.listCalls)
}

func TestCompetitionController_Create_BodyInvalido(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/teams/2/competitions", `{"type":...`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Zero(t, svc.createCalls)
}

func TestCompetitionController_Create_MaxAttemptsNoEntero(t *testing.T) {
	// El binding del DTO (*int64) rechaza max_attempts no-entero a 400 de Gin
	// antes de tocar el service (D1).
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 1)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/teams/2/competitions",
		`{"type":"route","start_at":"2026-10-01T00:00:00Z","participants":[3,4],"max_attempts":1.5}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Zero(t, svc.createCalls)
}

func TestCompetitionService_Create_Success_MapeaDatos(t *testing.T) {
	svc := &mockCompetitionService{createFn: func(callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error) {
		metric := "distance"
		return &competition.CompetitionResponse{ID: 9, TeamID: teamID, Type: competition.TypeMetric, Metric: &metric,
			Status: competition.StatusActive, Participants: []int64{3, 4}}, nil
	}}
	router := setupCompetitionRouterS(t, svc, 1)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/teams/2/competitions",
		`{"type":"metric","metric":"distance","start_at":"2026-10-01T00:00:00Z","end_at":"2026-10-10T00:00:00Z","participants":[3,4]}`)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, 1, svc.createCalls)
	var out competition.CompetitionDetailResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	assert.Equal(t, int64(9), out.Data.ID)
	assert.Equal(t, "metric", out.Data.Type)
	require.Len(t, out.Data.Participants, 2)
}

func TestCompetitionController_ErroresMapeados(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{"body invalido", services.ErrCompetitionInvalid, http.StatusBadRequest, "Bad request"},
		{"transicion invalida", services.ErrCompetitionConflict, http.StatusBadRequest, "Bad request"},
		{"ajeno", services.ErrCompetitionForbidden, http.StatusForbidden, "Forbidden"},
		{"no existe", daos.ErrCompetitionNotFound, http.StatusNotFound, "Not Found"},
	}
	for _, tc := range cases {
		svc := &mockCompetitionService{closeFn: func(callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error) {
			return nil, nil, tc.serviceErr
		}}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/close", "")
		require.Equal(t, tc.wantStatus, resp.Code, tc.name)
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
		assert.Equal(t, tc.wantCode, out["code"], tc.name)
	}

	// Error crudo del DAO → 500 genérico sin leak.
	svc := &mockCompetitionService{listFn: func(callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error) {
		return nil, errors.New("SQLSTATE 23505 detalle interno")
	}}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "SQLSTATE")
}

func TestCompetitionController_ListQueryParams(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions?status=active&group_id=9", "")
	require.Equal(t, 1, svc.listCalls)
	assert.Equal(t, "active", svc.lastStatus)
	require.NotNil(t, svc.lastGroupID)
	assert.Equal(t, int64(9), *svc.lastGroupID)
}

func TestCompetitionController_RejectBodyOpcional(t *testing.T) {
	// Con reason.
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/reject", `{"reason":"demasiado difícil"}`)
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, svc.lastReason)
	assert.Equal(t, "demasiado difícil", *svc.lastReason)

	// Sin body: igual rechaza con reason nil.
	svc = &mockCompetitionService{}
	router = setupCompetitionRouterS(t, svc, 7)
	resp = doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/reject", "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Nil(t, svc.lastReason)
}

func TestCompetitionController_Cancel_Mensaje(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodDelete, "/api/v1/competitions/2/cancel", "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, competition.MsgCompetitionCancelled, competitionErrorMessage(t, resp))
}

// FailingDB no-leak pin (D12): el DAO de guard falla con Postgres real y el
// endpoint de listado responde 500 genérico sin filtrar el error inyectado.
func TestCompetitionController_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	realSvc := services.NewCompetitionService(daos.NewCompetitionDao(failing), nil, nil)
	router := setupCompetitionRouterS(t, realSvc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, resp.Body.String(), "FailingDB")
}

func TestCompetitionController_GetDetail_Success(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/2", "")
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, 1, svc.getCalls)
	var out competition.CompetitionDetailResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	assert.Equal(t, int64(2), out.Data.ID)
}

// E4.2: control:competition_finished por los canales session:{instance} de los
// intentos registrados. Hub real mismo harness de los broadcast de feedback.
type competitionFinishedFrame struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func TestCompetitionController_Close_WSControlFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := realtime.NewHub()
	gateway := realtime.NewGateway(hub, permissiveSessionAuthorizer{}, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, int64(7))
		c.Next()
	})
	router.GET("/ws", func(c *gin.Context) {
		conn, err := gateway.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		gateway.Serve(conn, 7)
	})

	svc := &mockCompetitionService{closeFn: func(callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error) {
		return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusFinished}, []int64{42, 43}, nil
	}}
	router.PATCH("/api/v1/competitions/:id/close", NewCompetitionController(svc, realtime.NewHubNotifier(hub)).Close)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	// Cliente suscripto al canal del intento 42.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	frames := make(chan [2]string, 16)
	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, raw, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			var f competitionFinishedFrame
			if uerr := json.Unmarshal(raw, &f); uerr == nil {
				frames <- [2]string{f.Type, string(f.Payload)}
			}
		}
	}()
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "subscribe", "channel": "session:42"}))

	// El frame del close solo llega a clientes ya registrados en el hub:
	// esperar el "subscribed" antes del write (sin esto, bajo carga, el
	// PATCH le gana al subscribe y el test flakea).
waitForSubscribed:
	for {
		select {
		case <-time.After(2 * time.Second):
			t.Fatal("timeout esperando subscribed")
		case got := <-frames:
			if got[0] == "subscribed" {
				break waitForSubscribed
			}
		}
	}

	require.Equal(t, http.StatusOK, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/5/close", "").Code)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout esperando control:competition_finished")
		case got := <-frames:
			if got[0] == "control:competition_finished" {
				assert.Contains(t, got[1], `"competitionId":5`)
				return
			}
		}
	}
}

func TestCompetitionController_Close_SinIntentos_Silencio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := realtime.NewHub()
	gateway := realtime.NewGateway(hub, permissiveSessionAuthorizer{}, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, int64(7))
		c.Next()
	})
	router.GET("/ws", func(c *gin.Context) {
		conn, err := gateway.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		gateway.Serve(conn, 7)
	})
	svc := &mockCompetitionService{closeFn: func(callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error) {
		return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusFinished}, nil, nil
	}}
	router.PATCH("/api/v1/competitions/:id/close", NewCompetitionController(svc, realtime.NewHubNotifier(hub)).Close)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	subs := make(chan [2]string, 16)
	go func() {
		for {
			_, raw, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			var f competitionFinishedFrame
			if uerr := json.Unmarshal(raw, &f); uerr == nil {
				subs <- [2]string{f.Type, string(f.Payload)}
			}
		}
	}()
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "subscribe", "channel": "session:42"}))
	require.Equal(t, http.StatusOK, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/5/close", "").Code)

	_ = subs
	select {
	case got := <-subs:
		require.NotEqual(t, "control:competition_finished", got[0], "sin intentos no se emite el control")
	case <-time.After(300 * time.Millisecond):
	}
}

// FailingDB no-leak del close (E4.2 D12): la primera query del cierre falla
// con DB real inyectada y responde 500 genérico sin filtrar el error.
func TestCompetitionController_Close_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	realSvc := services.NewCompetitionService(daos.NewCompetitionDao(failing), nil, nil)
	router := setupCompetitionRouterS(t, realSvc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/5/close", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "inyectado")
	assert.NotContains(t, resp.Body.String(), "FailingDB")
}

// Intake de intentos (competencias-ruta-intake D2): guard order del controller
// y shape 201 partag typical {message, data} del POST runner.
func TestCompetitionController_CreateAttempt(t *testing.T) {
	instanceID := int64(42)

	t.Run("201 happy", func(t *testing.T) {
		svc := &mockCompetitionService{}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusCreated, resp.Result().StatusCode)
		var body struct {
			Message string `json:"message"`
			Data    struct {
				ID                int64  `json:"id"`
				SessionInstanceID int64  `json:"session_instance_id"`
				Status            string `json:"status"`
				CompetitionID     *int64 `json:"competition_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, "intento registrado", body.Message)
		assert.Equal(t, instanceID, body.Data.SessionInstanceID)
		assert.Equal(t, "wip", body.Data.Status)
		require.NotNil(t, body.Data.CompetitionID)
		assert.Equal(t, int64(5), *body.Data.CompetitionID)
		assert.Equal(t, 1, svc.attemptCalls)
	})

	t.Run("400 type no route", func(t *testing.T) {
		svc := &mockCompetitionService{}
		svc.createAttempt = func(callerID, competitionID int64) (*dbs.RunnerSession, error) {
			return nil, fmt.Errorf("%w: solo las competencias de ruta admiten intentos", services.ErrCompetitionInvalid)
		}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusBadRequest, resp.Result().StatusCode)
		assert.Contains(t, competitionErrorMessage(t, resp), "solo las competencias de ruta admiten intentos")
	})

	t.Run("403 no participante", func(t *testing.T) {
		svc := &mockCompetitionService{}
		svc.createAttempt = func(callerID, competitionID int64) (*dbs.RunnerSession, error) {
			return nil, services.ErrCompetitionForbidden
		}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusForbidden, resp.Result().StatusCode)
	})

	t.Run("404 inexistente", func(t *testing.T) {
		svc := &mockCompetitionService{}
		svc.createAttempt = func(callerID, competitionID int64) (*dbs.RunnerSession, error) {
			return nil, daos.ErrCompetitionNotFound
		}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusNotFound, resp.Result().StatusCode)
	})

	t.Run("400 límite de intentos", func(t *testing.T) {
		svc := &mockCompetitionService{}
		svc.createAttempt = func(callerID, competitionID int64) (*dbs.RunnerSession, error) {
			return nil, fmt.Errorf("%w: %s", services.ErrCompetitionInvalid, daos.ErrCompetitionAttemptLimit.Error())
		}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusBadRequest, resp.Result().StatusCode)
		assert.Contains(t, competitionErrorMessage(t, resp), "llegaste al límite de intentos de esta competencia")
	})

	t.Run("path no numérico", func(t *testing.T) {
		svc := &mockCompetitionService{}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/abc/attempts", "")
		require.Equal(t, http.StatusBadRequest, resp.Result().StatusCode)
		assert.Zero(t, svc.attemptCalls)
	})
}

// Guard order del handler: la respuesta del service llega tal cual; el error
// genérico (db caída) no queda en el body con detalles internos.
func TestCompetitionController_CreateAttempt_InternalErrorNoLeak(t *testing.T) {
	svc := &mockCompetitionService{}
	svc.createAttempt = func(callerID, competitionID int64) (*dbs.RunnerSession, error) {
		return nil, errors.New("punto de falla interno")
	}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/competitions/5/attempts", "")
	require.Equal(t, http.StatusInternalServerError, resp.Result().StatusCode)
	assert.Contains(t, competitionErrorMessage(t, resp), "error interno")
	assert.NotContains(t, resp.Body.String(), "punto de falla interno")
}

// ListAttempts (Gap 36 D4): parsing de query, default caller, mapping de
// errores y shape 200 {data: [...]} sin has_unseen.
func TestCompetitionController_ListAttempts(t *testing.T) {
	attemptItem := competition.CompetitionAttemptItem{
		ID: 1, SessionInstanceID: 42, Status: "wip",
		StartedAt: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC),
	}

	t.Run("200 sin query default caller", func(t *testing.T) {
		svc := &mockCompetitionService{listAttempts: func(callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error) {
			return &competition.CompetitionAttemptsResponse{Data: []competition.CompetitionAttemptItem{attemptItem}}, nil
		}}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusOK, resp.Result().StatusCode)
		require.Equal(t, 1, svc.listAttemptsCalls)
		assert.Equal(t, int64(7), svc.lastAthleteUserID, "sin query el atleta es el caller")

		var out competition.CompetitionAttemptsResponse
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
		require.Len(t, out.Data, 1)
		assert.Equal(t, int64(1), out.Data[0].ID)
		assert.Equal(t, int64(42), out.Data[0].SessionInstanceID)
		assert.Equal(t, "wip", out.Data[0].Status)

		// La envoltura es solo data: sin has_unseen.
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &raw))
		require.Len(t, raw, 1)
		require.Contains(t, raw, "data")
	})

	t.Run("con query de otro atleta", func(t *testing.T) {
		svc := &mockCompetitionService{}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts?athlete_user_id=9", "")
		require.Equal(t, http.StatusOK, resp.Result().StatusCode)
		assert.Equal(t, int64(9), svc.lastAthleteUserID)
	})

	t.Run("400 query invalida", func(t *testing.T) {
		svc := &mockCompetitionService{}
		router := setupCompetitionRouterS(t, svc, 7)
		for _, q := range []string{"?athlete_user_id=abc", "?athlete_user_id=-1", "?athlete_user_id=0"} {
			resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts"+q, "")
			require.Equal(t, http.StatusBadRequest, resp.Result().StatusCode, q)
		}
		assert.Zero(t, svc.listAttemptsCalls)
	})

	t.Run("400 path no numerico", func(t *testing.T) {
		svc := &mockCompetitionService{}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/abc/attempts", "")
		require.Equal(t, http.StatusBadRequest, resp.Result().StatusCode)
		assert.Zero(t, svc.listAttemptsCalls)
	})

	t.Run("403 y 404 mapeados", func(t *testing.T) {
		svc := &mockCompetitionService{listAttempts: func(callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error) {
			return nil, services.ErrCompetitionForbidden
		}}
		router := setupCompetitionRouterS(t, svc, 7)
		assert.Equal(t, http.StatusForbidden, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts", "").Result().StatusCode)

		svc = &mockCompetitionService{listAttempts: func(callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error) {
			return nil, daos.ErrCompetitionNotFound
		}}
		router = setupCompetitionRouterS(t, svc, 7)
		assert.Equal(t, http.StatusNotFound, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts", "").Result().StatusCode)
	})

	t.Run("error crudo del service 500 sin leak", func(t *testing.T) {
		svc := &mockCompetitionService{listAttempts: func(callerID, competitionID, athleteUserID int64) (*competition.CompetitionAttemptsResponse, error) {
			return nil, errors.New("fuga filtrada")
		}}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts", "")
		require.Equal(t, http.StatusInternalServerError, resp.Result().StatusCode)
		assert.NotContains(t, resp.Body.String(), "fuga filtrada")
	})
}

// FailingDB no-leak del listado de intentos: la primera query del service
// falla con DB real y responde 500 genérico sin filtrar el error inyectado.
func TestCompetitionController_ListAttempts_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	realSvc := services.NewCompetitionService(daos.NewCompetitionDao(failing), nil, nil)
	router := setupCompetitionRouterS(t, realSvc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/5/attempts", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, resp.Body.String(), "FailingDB")
}
