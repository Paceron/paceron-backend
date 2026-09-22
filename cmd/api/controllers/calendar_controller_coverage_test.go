package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
)

func TestMapCalendarError_RamasTipadas(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"sesion no encontrada", services.ErrCalendarSessionNotFound, 422, "sesión referenciada no encontrada"},
		{"formato horario invalido", services.ErrCalendarInvalidTimeFormat, 422, "presencial_time_from/presencial_time_to deben tener formato HH:MM"},
		{"rango horario invalido", services.ErrCalendarInvalidTimeRange, 422, "presencial_time_to debe ser posterior a presencial_time_from"},
		{"training sin instancia", services.ErrCalendarTrainingWithoutInstance, 422, services.ErrCalendarTrainingWithoutInstance.Error()},
		{"exclude_dates invalido", services.ErrCalendarInvalidDate, 422, services.ErrCalendarInvalidDate.Error()},
		{"colision presencial", services.ErrCalendarPresencialCollision, 409, "colisión presencial con otro equipo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg := mapCalendarError(tc.err)
			require.Equal(t, tc.wantCode, code)
			require.Equal(t, tc.wantMsg, msg)
		})
	}
}

func TestMemberCalendar_RangoVacioRespondeArrayVacio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockCalendarService{memberCalendarFn: func(ctx *gin.Context, userID int64, from, to time.Time) ([]calendar.AggregateCalendarDayResponse, error) {
		return []calendar.AggregateCalendarDayResponse{}, nil
	}}
	router := setupCalendarRouter(svc, 7)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/7/member-calendar?from=2026-03-01&to=2026-03-31", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body []json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Empty(t, body)
}
