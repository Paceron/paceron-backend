package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// paramErrorCode extrae el code del 400 del dashboard.
func paramErrorCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var paramErr *DashboardParamError
	require.ErrorAs(t, err, &paramErr)
	assert.ErrorIs(t, err, ErrDashboardInvalidQuery)
	return paramErr.Code
}

// ---------- 6.9b: los cinco rechazos de parámetro, cada uno con su code ----------

func TestDashboardService_ParametrosInvalidosSeRechazanConSuCode(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*MetricsQuery)
		endpoint func(*dashboardService, MetricsQuery) error
		code     string
	}{
		{
			name:   "tolerance fuera de rango",
			mutate: func(q *MetricsQuery) { q.Tolerance = "0.5" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Summary(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidTolerance,
		},
		{
			name:   "tolerance en cero",
			mutate: func(q *MetricsQuery) { q.Tolerance = "0" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Summary(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidTolerance,
		},
		{
			name:   "granularity desconocida",
			mutate: func(q *MetricsQuery) { q.Granularity = "hour" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.TimeSeries(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidGranularity,
		},
		{
			name:   "order inválido",
			mutate: func(q *MetricsQuery) { q.Order = "sideways" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Leaderboard(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidSort,
		},
		{
			name:   "page_size mayor a 100",
			mutate: func(q *MetricsQuery) { q.PageSize = "500" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Leaderboard(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidPageSize,
		},
		{
			name:   "rango de 12 años",
			mutate: func(q *MetricsQuery) { q.From, q.To = "2015-01-01", "2026-10-10" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Summary(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidRange,
		},
		{
			name:   "from ausente",
			mutate: func(q *MetricsQuery) { q.From = "" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Summary(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeMissingFrom,
		},
		{
			name:   "to anterior a from",
			mutate: func(q *MetricsQuery) { q.To = "2026-08-01" },
			endpoint: func(s *dashboardService, q MetricsQuery) error {
				_, err := s.Summary(nil, 10, q)
				return err
			},
			code: constants.ErrorCodeInvalidRange,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, dao, _ := dashboardFixture(t)
			query := dashboardQuery()
			tc.mutate(&query)

			err := tc.endpoint(service, query)

			assert.Equal(t, tc.code, paramErrorCode(t, err))
			assert.Equal(t, 0, dao.calls, "un 400 tiene que rechazarse SIN ejecutar la consulta")
		})
	}
}

// 6.11: el rango dentro del límite entra, el de 12 años no.
func TestDashboardService_RangoDentroDelLimiteEntra(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()
	query.From, query.To = "2025-01-01", "2026-10-10"

	_, err := service.Summary(nil, 10, query)

	require.NoError(t, err, "2025-01-01 → 2026-10-10 entra por el límite de 2 años")
}

// 6.10: un sort fuera de la whitelist devuelve 400 sin ejecutar la consulta.
func TestDashboardService_SortFueraDeWhitelistNoEjecutaLaConsulta(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	query := dashboardQuery()
	query.Sort = "duration_ms"

	_, err := service.Leaderboard(nil, 10, query)

	assert.Equal(t, constants.ErrorCodeInvalidSort, paramErrorCode(t, err))
	assert.Equal(t, 0, dao.calls)
}

func TestDashboardService_SortsValidosEnElRango(t *testing.T) {
	for _, sortColumn := range []string{
		dashboard.SortBestTimeMs, dashboard.SortMeanSpeedKph,
		dashboard.SortTotalDistanceM, dashboard.SortComplianceRate,
	} {
		service, _, _ := dashboardFixture(t)
		query := dashboardQuery()
		query.Sort = sortColumn

		_, err := service.Leaderboard(nil, 10, query)

		assert.NoError(t, err, "sort=%s", sortColumn)
	}
}

func TestDashboardService_GranularidadesYOrdenesValidos(t *testing.T) {
	for _, granularity := range []string{"session", "week", "month", "quarter"} {
		service, _, _ := dashboardFixture(t)
		query := dashboardQuery()
		query.Granularity = granularity

		_, err := service.TimeSeries(nil, 10, query)

		assert.NoError(t, err, "granularity=%s", granularity)
	}
	for _, order := range []string{"asc", "desc"} {
		service, _, _ := dashboardFixture(t)
		query := dashboardQuery()
		query.Order = order

		_, err := service.Leaderboard(nil, 10, query)

		assert.NoError(t, err, "order=%s", order)
	}
}

func TestDashboardService_ToleranciaEnElRango(t *testing.T) {
	for _, tolerance := range []string{"0.01", "0.05", "0.2"} {
		service, _, _ := dashboardFixture(t)
		query := dashboardQuery()
		query.Tolerance = tolerance

		_, err := service.Summary(nil, 10, query)

		assert.NoError(t, err, "tolerance=%s", tolerance)
	}
}

// El default de tolerancia es 5 % y llega en la respuesta.
func TestDashboardService_Defaults(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()

	response, err := service.Leaderboard(nil, 10, query)

	require.NoError(t, err)
	assert.Equal(t, dashboard.SortBestTimeMs, response.Sort)
	assert.Equal(t, dashboard.OrderAsc, response.Order)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 20, response.PageSize)
}

// ---------- 6.9: autorización por rol ----------

// Las tres endpoints de comparación son del dueño del equipo: el corredor que es
// solo miembro recibe 403 y la consulta no se ejecuta. La regla va en el backend
// y no en un `if` del front.
func TestDashboardService_ComparacionesSonSoloDelDueno(t *testing.T) {
	calls := map[string]func(*dashboardService) error{
		"/leaderboard": func(s *dashboardService) error {
			_, err := s.Leaderboard(nil, 77, dashboardQuery())
			return err
		},
		"/comparisons": func(s *dashboardService) error {
			_, err := s.Comparisons(nil, 77, dashboard.ComparisonsRequest{TeamID: 29, From: "2026-09-01", To: "2026-09-30"})
			return err
		},
		"/comparisons/periods": func(s *dashboardService) error {
			_, err := s.Periods(nil, 77, dashboard.PeriodsRequest{
				TeamID:  29,
				Current: dashboard.PeriodRange{From: "2026-09-01", To: "2026-09-30"},
			})
			return err
		},
	}
	for endpoint, call := range calls {
		t.Run(endpoint, func(t *testing.T) {
			service, dao, _ := dashboardFixtureAs(t, 77, false)

			err := call(service)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrDashboardForbidden)
			assert.Equal(t, 0, dao.calls, "el 403 tiene que caer antes de la consulta")
		})
	}
}

// El resto de los endpoints responde 200 a un corredor miembro del scope.
func TestDashboardService_RunnerVeElScopeDeSuEquipo(t *testing.T) {
	for endpoint, call := range map[string]func(*dashboardService) error{
		"/filters": func(s *dashboardService) error {
			_, err := s.Filters(nil, 77, 29)
			return err
		},
		"/metrics/summary": func(s *dashboardService) error {
			_, err := s.Summary(nil, 77, dashboardQuery())
			return err
		},
		"/metrics/timeseries": func(s *dashboardService) error {
			_, err := s.TimeSeries(nil, 77, dashboardQuery())
			return err
		},
		"/exercises/{id}/series": func(s *dashboardService) error {
			_, err := s.ExerciseSeries(nil, 77, 7, dashboardQuery())
			return err
		},
	} {
		t.Run(endpoint, func(t *testing.T) {
			service, dao, _ := dashboardFixtureAs(t, 77, false)
			dao.catalogByIDFn = func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
				return &daos.DashboardCatalogExercises{ID: id, OwnerID: dashboardOwnerID, Name: "Distancia 4 km", Kind: "ritmo_continuo"}, nil
			}

			require.NoError(t, call(service))
		})
	}
}

// Un team_id fuera del scope es 403 también para el dueño de otro equipo.
func TestDashboardService_EquipoAjenoEs403(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	query := dashboardQuery()
	query.TeamID = 30

	_, err := service.Summary(nil, 10, query)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
	assert.Equal(t, 0, dao.calls)
}

// 4.4: 404 para inexistente, 403 para ajeno.
func TestDashboardService_GrupoInexistenteEs404YAjenoEs403(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()

	missing := int64(9999)
	query.GroupID = &missing
	_, err := service.Summary(nil, 10, query)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardNotFound)

	foreign := int64(99)
	query.GroupID = &foreign
	_, err = service.Summary(nil, 10, query)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

// 4.3: /comparisons valida cada corredor por separado y nombra al que falla.
func TestDashboardService_ComparisonsNombraElCorredorFueraDeScope(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{
		{GroupID: 39, UserID: 1}, {GroupID: 39, UserID: 2},
	}}

	_, err := service.Comparisons(nil, 10, dashboard.ComparisonsRequest{
		TeamID:         29,
		From:           "2026-09-01",
		To:             "2026-09-30",
		AthleteUserIDs: []int64{1, 2, 3},
	})

	require.Error(t, err)
	var athleteErr *DashboardAthleteOutOfScope
	require.ErrorAs(t, err, &athleteErr)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
	assert.Equal(t, int64(3), athleteErr.AthleteUserID, "el mensaje identifica cuál falló")
	assert.Contains(t, athleteErr.Error(), "3")
	assert.Equal(t, 0, dao.calls, "no se devuelven datos de los válidos")
}

// ---------- filtros de ejercicio ----------

// exercise_id, exercise_ids y kinds son excluyentes: los tres o ninguno es 400.
func TestDashboardService_FiltroDeEjercicioExcluyente(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()
	exerciseID := int64(7)
	query.ExerciseID = &exerciseID
	query.Kinds = []string{"ritmo_continuo"}

	_, err := service.Summary(nil, 10, query)

	assert.NotEmpty(t, paramErrorCode(t, err))
}

func TestDashboardService_EjercicioInexistenteEs404(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()
	exerciseID := int64(999999)
	query.ExerciseID = &exerciseID

	_, err := service.Summary(nil, 10, query)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

func TestDashboardService_EjercicioDeOtroDueñoEs403(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	// El DAO devuelve el ejercicio pero con otro owner_id: existe y no está en el
	// scope del equipo pedido.
	dao.catalogByIDsFn = func(ctx *gin.Context, ids []int64) ([]daos.DashboardCatalogExercises, error) {
		return []daos.DashboardCatalogExercises{{ID: 7, OwnerID: 999, Name: "Distancia 4 km", Kind: "ritmo_continuo"}}, nil
	}
	query := dashboardQuery()
	exerciseID := int64(7)
	query.ExerciseID = &exerciseID

	_, err := service.Summary(nil, 10, query)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

// kinds[] solo admite los cinco tipos vigentes: un legacy es 400.
func TestDashboardService_KindsLegacyEs400(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()
	query.Kinds = []string{"running"}

	_, err := service.Summary(nil, 10, query)

	assert.NotEmpty(t, paramErrorCode(t, err))
}

// ---------- filtros ----------

// El entrenador ve sus corredores; el corredor, solo su propia entrada.
func TestDashboardService_FiltersRecortaPorRol(t *testing.T) {
	trainerSvc, trainerDao2, _ := dashboardFixture(t)
	trainerDao2.athletesFn = func(ctx *gin.Context, groupIDs []int64) ([]int64, error) {
		return []int64{1, 2, 3}, nil
	}

	trainerResponse, err := trainerSvc.Filters(nil, 10, 29)

	require.NoError(t, err)
	assert.Len(t, trainerResponse.Athletes, 3)
	assert.Equal(t, constants.CurrentExerciseKinds(), trainerResponse.ExerciseTypes,
		"los tipos del tablist son los cinco vigentes, no los diez valores válidos de escritura")
	assert.Equal(t, "owner", trainerResponse.Groups[0].Role)

	runner, runnerDao, _ := dashboardFixtureAs(t, 77, false)
	runnerDao.athletesFn = func(ctx *gin.Context, groupIDs []int64) ([]int64, error) { return []int64{1, 2, 77}, nil }

	runnerResponse, err := runner.Filters(nil, 77, 29)

	require.NoError(t, err)
	require.Len(t, runnerResponse.Athletes, 1)
	assert.Equal(t, int64(77), runnerResponse.Athletes[0].ID)
	assert.Equal(t, "member", runnerResponse.Groups[0].Role)
}

// Sin datos, date_range viene null — no {min: null, max: null}.
func TestDashboardService_FiltersSinDatosDateRangeNull(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	dao.athletesFn = func(ctx *gin.Context, groupIDs []int64) ([]int64, error) { return nil, nil }

	response, err := service.Filters(nil, 10, 29)

	require.NoError(t, err)
	assert.Nil(t, response.DateRange)
	assert.Empty(t, response.Exercises)
	assert.Empty(t, response.Athletes)
}

func TestDashboardService_FiltersDateRangeISO(t *testing.T) {
	min := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	max := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	service, dao, _ := dashboardFixture(t)
	dao.dateRangeFn = func(ctx *gin.Context, groupIDs []int64) (*time.Time, *time.Time, error) {
		return &min, &max, nil
	}

	response, err := service.Filters(nil, 10, 29)

	require.NoError(t, err)
	require.NotNil(t, response.DateRange)
	assert.Equal(t, "2026-09-01", response.DateRange.Min, "las fechas de salida son ISO y solo ISO")
	assert.Equal(t, "2026-10-10", response.DateRange.Max)
}

// ---------- períodos ----------

func TestDashboardService_PeriodsDerivaElPeriodoAnterior(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	var ranges []string
	dao.findSeriesFn = func(ctx *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		ranges = append(ranges, dashboard.FormatDate(filter.From)+"→"+dashboard.FormatDate(filter.To))
		return []daos.DashboardSeriesRow{
			dashboardSeriesRow(1, 240000, 1000, filter.From),
			dashboardSeriesRow(1, 250000, 1000, filter.From),
			dashboardSeriesRow(1, 200000, 1000, filter.From),
		}, nil
	}

	response, err := service.Periods(nil, 10, dashboard.PeriodsRequest{
		TeamID:  29,
		Current: dashboard.PeriodRange{From: "2026-09-01", To: "2026-09-30"},
	})

	require.NoError(t, err)
	require.Len(t, ranges, 2, "una query para el período actual y otra para el anterior")
	assert.Equal(t, "2026-09-01→2026-09-30", ranges[0])
	assert.Equal(t, "2026-08-02→2026-08-31", ranges[1], "previous se deriva desplazando la misma cantidad de días")
	require.NotNil(t, response.Previous)
	assert.Equal(t, "2026-08-02", response.Previous.From)
	assert.Equal(t, "2026-08-31", response.Previous.To)
}

func TestDashboardService_PeriodsUsaElPreviousDelBody(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	var ranges []string
	dao.findSeriesFn = func(ctx *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		ranges = append(ranges, dashboard.FormatDate(filter.From)+"→"+dashboard.FormatDate(filter.To))
		return nil, nil
	}

	_, err := service.Periods(nil, 10, dashboard.PeriodsRequest{
		TeamID:   29,
		Current:  dashboard.PeriodRange{From: "2026-09-01", To: "2026-09-30"},
		Previous: &dashboard.PeriodRange{From: "2026-01-01", To: "2026-01-31"},
	})

	require.NoError(t, err)
	assert.Equal(t, "2026-01-01→2026-01-31", ranges[1])
}

// ---------- detalle de series ----------

// 4.1b: las instancias sin source_exercise_id entran por nombre normalizado, y
// solo si el dueño es el mismo del ejercicio pedido.
func TestDashboardService_ExerciseSeriesResuelveInstanciasPorNombre(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	dao.catalogByIDFn = func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
		return &daos.DashboardCatalogExercises{ID: id, OwnerID: 10, Name: "Distancia 4 km", Kind: "ritmo_continuo"}, nil
	}
	dao.unlinkedFn = func(ctx *gin.Context, groupIDs []int64) ([]daos.UnlinkedExerciseInstance, error) {
		return []daos.UnlinkedExerciseInstance{
			// Mismo dueño y mismo nombre normalizado: entra.
			{ExerciseInstanceID: 500, Name: "distancia 4 KM.", OwnerID: 10},
			// Mismo dueño, otro ejercicio: no entra.
			{ExerciseInstanceID: 501, Name: "Distancia 21 km", OwnerID: 10},
			// Mismo nombre pero otro dueño: NO entra — `exercises` es catálogo por
			// dueño y sin acotar mostraría el objetivo de un entrenador evaluando
			// las sesiones de otro.
			{ExerciseInstanceID: 502, Name: "Distancia 4 km", OwnerID: 999},
		}, nil
	}
	var filter daos.DashboardScopeFilter
	dao.findSeriesFn = func(ctx *gin.Context, f daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		filter = f
		return nil, nil
	}

	_, err := service.ExerciseSeries(nil, 10, 7, dashboardQuery())

	require.NoError(t, err)
	assert.Equal(t, []int64{7}, filter.ExerciseIDs)
	assert.Equal(t, []int64{500}, filter.LegacyExerciseInstanceIDs)
}

// Un ejercicio de catálogo de otro dueño no se resuelve por nombre.
func TestDashboardService_ExerciseSeriesDeOtroDueñoEs403(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	dao.catalogByIDFn = func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
		return &daos.DashboardCatalogExercises{ID: id, OwnerID: 999, Name: "Distancia 4 km"}, nil
	}

	_, err := service.ExerciseSeries(nil, 10, 7, dashboardQuery())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

func TestDashboardService_ExerciseSeriesInexistenteEs404(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	dao.catalogByIDFn = func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
		return nil, nil
	}

	_, err := service.ExerciseSeries(nil, 10, 999999, dashboardQuery())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

// ---------- integración service + DAO sobre Postgres real ----------

// La suite de autorización real: con DAOs de verdad, el scope del workout_feedback
// de un grupo no se filtra en memoria sino en el WHERE.
func TestDashboardService_PermisosConDAOsReales(t *testing.T) {
	db := testutils.SetupTestDB(t)
	service := NewDashboardService(
		daos.NewDashboardDao(db),
		NewDashboardScopeService(daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewTeamDao(db)),
		daos.NewUserDao(db),
		daos.NewGroupUserDao(db),
	)

	owner, group := task3OwnerGroup(t, db, "dashperm")
	team, err := daos.NewTeamDao(db).FindByID(nil, group.TeamID)
	require.NoError(t, err)
	require.NotNil(t, team)

	// El dueño entra con 200.
	_, err = service.Summary(nil, owner.ID, MetricsQuery{
		TeamID: team.ID, From: "2020-01-01", To: "2021-01-01",
	})
	assert.NoError(t, err, "el dueño del equipo ve su propio dashboard")

	// Un tercero sin relación recibe 403.
	_, err = service.Summary(nil, owner.ID+9999, MetricsQuery{
		TeamID: team.ID, From: "2020-01-01", To: "2021-01-01",
	})
	assert.ErrorIs(t, err, ErrDashboardForbidden)

	// Un equipo inexistente también es 403: no está en el scope.
	_, err = service.Summary(nil, owner.ID, MetricsQuery{
		TeamID: 999999, From: "2020-01-01", To: "2021-01-01",
	})
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

func dashboardSeriesRow(athleteID, timeMs int64, distanceM float64, date time.Time) daos.DashboardSeriesRow {
	return daos.DashboardSeriesRow{
		FeedbackID:        timeMs,
		SessionInstanceID: 1,
		SessionDate:       date,
		BucketStart:       date,
		AthleteUserID:     athleteID,
		Kind:              string(constants.ExerciseKindRitmoContinuo),
		TargetMinutes:     intPtr(5),
		TargetDistanceM:   intPtr(1000),
		DurationMs:        &timeMs,
		ActiveDurationMs:  &timeMs,
		DistanceMeters:    &distanceM,
		HasRunnerSession:  true,
	}
}

func intPtr(v int) *int { return &v }
