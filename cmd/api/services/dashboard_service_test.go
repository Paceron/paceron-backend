package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

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

// exercise_id, exercise_ids y kind son excluyentes: los tres o ninguno es 400.
func TestDashboardService_FiltroDeEjercicioExcluyente(t *testing.T) {
	service, _, _ := dashboardFixture(t)
	query := dashboardQuery()
	exerciseID := int64(7)
	query.ExerciseID = &exerciseID
	query.Kind = "ritmo_continuo"

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

// El filtro de tipo solo admite los tres tipos CON MÉTRICA. `liviano` y `elongacion`
// son tipos válidos del modelo, pero como bloques warmup/cooldown el filtro
// role='main' deja sus series todas afuera: ofrecerlos sería un filtro que siempre
// devuelve vacío, así que es un 400 y no un cero mudo.
func TestDashboardService_KindSinMetricaEs400(t *testing.T) {
	for _, kind := range []string{
		"liviano", "elongacion", // válidos del modelo, sin métrica
		"running", "walking", // legacy
		"volando", // inventado
	} {
		service, dao, _ := dashboardFixture(t)
		query := dashboardQuery()
		query.Kind = kind

		_, err := service.Summary(nil, 10, query)

		assert.Equal(t, constants.ErrorCodeInvalidRange, paramErrorCode(t, err), "kind=%s", kind)
		assert.Equal(t, 0, dao.calls, "un 400 se rechaza sin consultar")
	}
}

// Un kind con métrica se resuelve a los ids de catálogo de ese tipo. Antes esto
// devolvía 404 siempre: la comparación de "resolví todos los que pediste" corría
// contra la lista de ids —vacía con kind— y pedía que el set resuelto fuera de
// tamaño cero.
func TestDashboardService_KindConMetricaResuelveAlCatalogo(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	var askedKinds []string
	dao.catalogByOwnerFn = func(ctx *gin.Context, ownerID int64, kinds []string) ([]daos.DashboardCatalogExercises, error) {
		askedKinds = kinds
		return []daos.DashboardCatalogExercises{
			{ID: 7, OwnerID: dashboardOwnerID, Name: "Distancia 4 km", Kind: "ritmo_continuo"},
			{ID: 9, OwnerID: dashboardOwnerID, Name: "Distancia 21 km", Kind: "ritmo_continuo"},
		}, nil
	}
	var filter daos.DashboardScopeFilter
	dao.findSeriesFn = func(ctx *gin.Context, f daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		filter = f
		return nil, nil
	}
	query := dashboardQuery()
	query.Kind = "ritmo_continuo"

	_, err := service.Summary(nil, 10, query)

	require.NoError(t, err)
	assert.Equal(t, []string{"ritmo_continuo"}, askedKinds)
	assert.Equal(t, []int64{7, 9}, filter.ExerciseIDs, "los dos ejercicios del tipo llegan agregados en el backend")
}

// Un tipo del scope que no tiene ningún ejercicio de catálogo es 404, igual que un
// exercise_id inexistente: no es un filtro sin resultados.
func TestDashboardService_KindSinEjerciciosEs404(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	dao.catalogByOwnerFn = func(ctx *gin.Context, ownerID int64, kinds []string) ([]daos.DashboardCatalogExercises, error) {
		return nil, nil
	}
	query := dashboardQuery()
	query.Kind = "velocidad_distancia"

	_, err := service.Summary(nil, 10, query)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

// ---------- filtros de rango de datos ----------

// Los cuatro filtros de rango: no negativos y con el piso <= techo. Todos con
// INVALID_RANGE_FILTER y sin ejecutar la consulta. El decimal lo rechaza el
// controller, que es donde se parsea el string del query param.
func TestDashboardService_RangoDeSerieInvalidoEs400(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*MetricsQuery)
	}{
		{"distancia invertida", func(q *MetricsQuery) { q.DistanceFromM, q.DistanceToM = int64Ptr(42000), int64Ptr(5000) }},
		{"tiempo invertido", func(q *MetricsQuery) { q.TimeFromMin, q.TimeToMin = int64Ptr(60), int64Ptr(30) }},
		{"distancia negativa", func(q *MetricsQuery) { q.DistanceFromM = int64Ptr(-1) }},
		{"tiempo negativo", func(q *MetricsQuery) { q.TimeToMin = int64Ptr(-5) }},
		{"minutos imposibles de expresar en ms", func(q *MetricsQuery) { q.TimeFromMin = int64Ptr(9223372036854775807) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, dao, _ := dashboardFixture(t)
			query := dashboardQuery()
			tc.mutate(&query)

			_, err := service.Summary(nil, 10, query)

			assert.Equal(t, constants.ErrorCodeInvalidRangeFilter, paramErrorCode(t, err))
			assert.Equal(t, 0, dao.calls, "un 400 tiene que rechazarse SIN ejecutar la consulta")
		})
	}
}

// Los minutos del contrato se pasan al DAO en ms, que es la unidad con la que los KPIs
// comparan. Un rango de 4 minutos es 240000 ms, no 4.
func TestDashboardService_RangoDeTiempoLlegaEnMilisegundos(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	var filter daos.DashboardScopeFilter
	dao.findSeriesFn = func(ctx *gin.Context, f daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		filter = f
		return nil, nil
	}
	query := dashboardQuery()
	query.TimeFromMin, query.TimeToMin = int64Ptr(4), int64Ptr(90)
	query.DistanceFromM, query.DistanceToM = int64Ptr(5000), int64Ptr(42000)

	_, err := service.Summary(nil, 10, query)

	require.NoError(t, err)
	require.NotNil(t, filter.TimeFromMs)
	assert.Equal(t, int64(240000), *filter.TimeFromMs)
	require.NotNil(t, filter.TimeToMs)
	assert.Equal(t, int64(5400000), *filter.TimeToMs)
	require.NotNil(t, filter.DistanceFromM)
	assert.Equal(t, float64(5000), *filter.DistanceFromM)
	require.NotNil(t, filter.DistanceToM)
	assert.Equal(t, float64(42000), *filter.DistanceToM)
}

// El rango de un solo lado vale: pedir solo el piso deja entrar todo lo que lasted eso
// o más.
func TestDashboardService_RangoDeUnSoloLado(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	var filter daos.DashboardScopeFilter
	dao.findSeriesFn = func(ctx *gin.Context, f daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		filter = f
		return nil, nil
	}
	query := dashboardQuery()
	query.TimeFromMin = int64Ptr(30)

	_, err := service.Summary(nil, 10, query)

	require.NoError(t, err)
	require.NotNil(t, filter.TimeFromMs)
	assert.Equal(t, int64(1800000), *filter.TimeFromMs)
	assert.Nil(t, filter.TimeToMs, "sin el otro lado no hay techo")
	assert.Nil(t, filter.DistanceFromM)
	assert.Nil(t, filter.DistanceToM)
}

// El rango viaja a los seis endpoints: los tres GET y los dos POST comparten el mismo
// camino de validación y de consulta.
func TestDashboardService_RangoDeSerieLlegaATodosLosEndpoints(t *testing.T) {
	cases := map[string]func(*dashboardService) error{
		"/metrics/summary": func(s *dashboardService) error {
			_, err := s.Summary(nil, 10, rangedQuery())
			return err
		},
		"/metrics/timeseries": func(s *dashboardService) error {
			_, err := s.TimeSeries(nil, 10, rangedQuery())
			return err
		},
		"/leaderboard": func(s *dashboardService) error {
			_, err := s.Leaderboard(nil, 10, rangedQuery())
			return err
		},
		"/exercises/{id}/series": func(s *dashboardService) error {
			dao := s.dashboardDao.(*mockDashboardDao)
			dao.catalogByIDFn = func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
				return &daos.DashboardCatalogExercises{ID: id, OwnerID: dashboardOwnerID, Name: "Distancia 4 km"}, nil
			}
			_, err := s.ExerciseSeries(nil, 10, 7, rangedQuery())
			return err
		},
		"/comparisons": func(s *dashboardService) error {
			_, err := s.Comparisons(nil, 10, dashboard.ComparisonsRequest{
				TeamID: 29, From: "2026-09-01", To: "2026-09-30",
				DistanceFromM: int64Ptr(5000), TimeToMin: int64Ptr(90),
			})
			return err
		},
		"/comparisons/periods": func(s *dashboardService) error {
			_, err := s.Periods(nil, 10, dashboard.PeriodsRequest{
				TeamID: 29, Current: dashboard.PeriodRange{From: "2026-09-01", To: "2026-09-30"},
				DistanceFromM: int64Ptr(5000), TimeToMin: int64Ptr(90),
			})
			return err
		},
	}
	for endpoint, call := range cases {
		t.Run(endpoint, func(t *testing.T) {
			service, dao, _ := dashboardFixture(t)
			var filters []daos.DashboardScopeFilter
			dao.findSeriesFn = func(ctx *gin.Context, f daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
				filters = append(filters, f)
				return nil, nil
			}

			require.NoError(t, call(service))
			require.NotEmpty(t, filters, "el endpoint tiene que consultar series")
			for _, filter := range filters {
				require.NotNil(t, filter.DistanceFromM)
				assert.Equal(t, float64(5000), *filter.DistanceFromM)
				require.NotNil(t, filter.TimeToMs)
				assert.Equal(t, int64(90*60000), *filter.TimeToMs)
			}
		})
	}
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

// ---------- integración service + DAO sobre Postgres real ----------

// La serie que el rango excluye NO es una serie inválida: `invalid_series` sigue
// significando "no calculable". Con el rango aplicado en memoria —o contando lo
// descartado como inválido— el banner diría "tenés datos malos" cuando el usuario
// pidió una ventana más chica.
//
// El caso se arma para que las dos cosas sean distinguibles: hay una serie inválida de
// verdad (sin tiempo) DENTRO de la ventana de distancia, y dos series válidas
// Perfectamente calculables FUERA.
func TestDashboardService_LaSerieFueraDeRangoNoEsInvalida(t *testing.T) {
	db := testutils.SetupTestDB(t)
	service := NewDashboardService(
		daos.NewDashboardDao(db),
		NewDashboardScopeService(daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewTeamDao(db)),
		daos.NewUserDao(db),
		daos.NewGroupUserDao(db),
	)
	scope := newDashboardRealScope(t, db, "rangoinv")

	// 6 km en 7 min: dentro. 50 km: fuera por distancia. Sin ningún tiempo: inválida
	// de verdad, y con 6 km para que la ventana de distancia la alcance.
	dashboardRealFeedback(t, db, scope, 1, int64Ptr(420000), int64Ptr(420000), 6000)
	dashboardRealFeedback(t, db, scope, 2, int64Ptr(600000), int64Ptr(600000), 50000)
	dashboardRealFeedback(t, db, scope, 3, nil, nil, 6000)

	query := MetricsQuery{
		TeamID: scope.teamID, From: "2026-09-14", To: "2026-09-16",
		DistanceFromM: int64Ptr(5000), DistanceToM: int64Ptr(42000),
	}

	full, err := service.Summary(nil, scope.ownerID, queryWithoutSeriesRange(query))
	require.NoError(t, err)
	assert.Equal(t, 3, full.Totals.Series)
	assert.Equal(t, 2, full.Totals.ValidSeries)
	assert.Equal(t, 1, full.DataQuality.InvalidSeries)

	ranged, err := service.Summary(nil, scope.ownerID, query)
	require.NoError(t, err)
	assert.Equal(t, 2, ranged.Totals.Series, "entran la de 6 km y la inválida, que está dentro de la ventana")
	assert.Equal(t, 1, ranged.Totals.ValidSeries)
	assert.Equal(t, 1, ranged.DataQuality.InvalidSeries,
		"la serie de 50 km es válida y quedó afuera: no puede contar como inválida")
	assert.True(t, ranged.HasTarget, "el rango no invalida el objetivo: viene del ejercicio, no de la serie")
}

func queryWithoutSeriesRange(query MetricsQuery) MetricsQuery {
	query.DistanceFromM, query.DistanceToM = nil, nil
	query.TimeFromMin, query.TimeToMin = nil, nil
	return query
}

// dashboardRealScope arma el scope mínimo del dashboard contra Postgres real: dueño,
// equipo, grupo, corredor con membresía, día de calendario y bloque principal con
// objetivo congelado.
type dashboardRealScope struct {
	ownerID   int64
	teamID    int64
	groupID   int64
	athleteID int64
	sessionID int64
	mainEx    int64
	date      time.Time
}

func newDashboardRealScope(t *testing.T, db *gorm.DB, tag string) *dashboardRealScope {
	t.Helper()
	owner, group := task3OwnerGroup(t, db, tag)
	athlete := &dbs.User{Name: "Rango", Surname: tag, Email: "rango-" + tag + "@test.com", DNI: "5099" + tag}
	athlete.BirthDate = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(athlete).Error)

	date := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	session := &dbs.SessionInstance{Name: "sesion " + tag}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &session.ID,
	}))
	require.NoError(t, daos.NewGroupUserDao(db).Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: athlete.ID, DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}))

	catalog := &dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(30), DistanceM: intPtr(4000)}
	require.NoError(t, db.Create(catalog).Error)
	instance := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(30), DistanceM: intPtr(4000), SourceExerciseID: &catalog.ID}
	require.NoError(t, db.Create(instance).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: session.ID, ExerciseInstanceID: instance.ID,
		Role: string(constants.SessionExerciseRoleMain),
	}).Error)

	return &dashboardRealScope{
		ownerID: owner.ID, teamID: group.TeamID, groupID: group.ID, athleteID: athlete.ID,
		sessionID: session.ID, mainEx: instance.ID, date: date,
	}
}

func dashboardRealFeedback(t *testing.T, db *gorm.DB, scope *dashboardRealScope, setNumber int, active, duration *int64, distanceM float64) {
	t.Helper()
	completed := "completa"
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID:              &scope.teamID,
		AssignedSessionID:   scope.sessionID,
		AssignedExerciseID:  scope.mainEx,
		AthleteUserID:       scope.athleteID,
		FeedbackOwnerUserID: scope.athleteID,
		ReportSource:        "atleta",
		SessionDate:         scope.date,
		SetNumber:           setNumber,
		DurationMs:          duration,
		ActiveDurationMs:    active,
		DistanceMeters:      &distanceM,
		CompletionStatus:    &completed,
		MediaURLs:           pgtype.TextArray{Status: pgtype.Present},
	}).Error)
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

// rangedQuery es la query del período con dos lados de cada magnitud de rango.
func rangedQuery() MetricsQuery {
	query := dashboardQuery()
	query.DistanceFromM, query.DistanceToM = int64Ptr(5000), int64Ptr(42000)
	query.TimeFromMin, query.TimeToMin = int64Ptr(4), int64Ptr(90)
	return query
}
