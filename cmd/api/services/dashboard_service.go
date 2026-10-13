package services

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// ErrDashboardInvalidQuery envuelve todo 400 de parámetro del dashboard.
var ErrDashboardInvalidQuery = fmt.Errorf("dashboard: parámetro inválido")

// DashboardParamError es el 400 con code estable. El cliente no debe ofrecer
// reintentar sin cambiar la request: por eso viaja el code y no solo el mensaje.
type DashboardParamError struct {
	Code    string
	Message string
}

func (e *DashboardParamError) Error() string { return e.Message }

func (e *DashboardParamError) Unwrap() error { return ErrDashboardInvalidQuery }

func newDashboardParamError(code, format string, args ...interface{}) error {
	return &DashboardParamError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// DashboardAthleteForbidden nombra al corredor fuera de scope en el 403 de
// /comparisons, sin devolver datos de los válidos.
func newDashboardAthleteForbidden(athleteUserID int64) error {
	return &DashboardAthleteOutOfScope{AthleteUserID: athleteUserID}
}

// Límites de las consultas analíticas (design.md D12). Acotan el peor caso de cada
// request individual: no hay rate limiter en el backend, y agregar uno sería
// dependencia o patrón nuevo.
const (
	// dashboardMaxRangeDays es el rango máximo en días.
	dashboardMaxRangeDays = 731
	// dashboardMaxPageSize es el tope de page_size.
	dashboardMaxPageSize = 100
	// dashboardMinTolerance / dashboardMaxTolerance acotan la tolerancia para que
	// nadie la use como atajo de "no compares nada" (design.md D5).
	dashboardMinTolerance = 0.01
	dashboardMaxTolerance = 0.20

	dashboardDefaultPage     = 1
	dashboardDefaultPageSize = 20
	dashboardDefaultSort     = dashboard.SortBestTimeMs
	dashboardDefaultOrder    = dashboard.OrderAsc
	// El default de sort tiene que ser best_time_ms con orden ascendente porque con
	// mean_speed_kph el ascendente pondría PRIMERO al más lento, que es lo
	// contrario de un ranking.
	dashboardDefaultGranularity = "session"
)

// dashboardGranularities son los cuatro anclajes de bucket admitidos.
var dashboardGranularities = map[string]bool{
	dashboardDefaultGranularity: true,
	"week":                      true,
	"month":                     true,
	"quarter":                   true,
}

// dashboardOrders son los dos sentidos admitidos.
var dashboardOrders = map[string]bool{
	dashboard.OrderAsc:  true,
	dashboard.OrderDesc: true,
}

// MetricsQuery son los parámetros crudos de los endpoints de métricas, tal como
// llegan del controller. La validación vive en el service y no en el DAO (el
// precedente del repo es historySortWhitelist en
// workout_feedback_service.go:661): el DAO no tiene que saber qué es una whitelist
// para no poder ejecutar una consulta con un ORDER BY inyectado.
type MetricsQuery struct {
	TeamID     int64
	GroupID    *int64
	ExerciseID *int64
	// ExerciseIDs y Kinds llegan del body en los POST; en GET los arma el
	// controller desde los query params repetidos.
	ExerciseIDs    []int64
	Kinds          []string
	From           string
	To             string
	Granularity    string
	Tolerance      string
	Page           string
	PageSize       string
	Sort           string
	Order          string
	AthleteUserIDs []int64
}

// validatedMetricsQuery es la query ya normalizada: defaults aplicados, whitelist
// validada y fechas parseadas. A partir de acá no queda nada por rechazar.
type validatedMetricsQuery struct {
	// Scope es el scope ya resuelto: se calcula una vez por request y se pasa ya
	// resuelto a los DAOs, de modo que ningún DAO pueda aceptar un group_id crudo.
	Scope       *DashboardScope
	CallerID    int64
	TeamID      int64
	GroupIDs    []int64
	ExerciseIDs []int64
	// LegacyExerciseInstanceIDs son instancias resueltas por nombre normalizado
	// (tarea 4.1b): sin ellas, /exercises/{id}/series perdería en silencio todo el
	// histórico anterior al change de instancia-referencia-catálogo.
	LegacyExerciseInstanceIDs []int64
	Kinds                     []string
	From                      time.Time
	To                        time.Time
	Granularity               string
	Tolerance                 float64
	Page                      int
	PageSize                  int
	Sort                      string
	Order                     string
	AthleteUserIDs            []int64
}

// DashboardServiceInterface expone los siete endpoints del dashboard.
type DashboardServiceInterface interface {
	Filters(ctx *gin.Context, callerID, teamID int64) (*dashboard.FiltersResponse, error)
	Summary(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.SummaryResponse, error)
	TimeSeries(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.TimeseriesResponse, error)
	Leaderboard(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.LeaderboardResponse, error)
	ExerciseSeries(ctx *gin.Context, callerID int64, catalogExerciseID int64, query MetricsQuery) (*dashboard.SeriesResponse, error)
	Comparisons(ctx *gin.Context, callerID int64, request dashboard.ComparisonsRequest) (*dashboard.ComparisonsResponse, error)
	Periods(ctx *gin.Context, callerID int64, request dashboard.PeriodsRequest) (*dashboard.PeriodsResponse, error)
}

type dashboardService struct {
	dashboardDao daos.DashboardDaoInterface
	scopeService DashboardScopeServiceInterface
	userDao      daos.UserDaoInterface
	groupUserDao daos.GroupUserDaoInterface
}

func NewDashboardService(
	dashboardDao daos.DashboardDaoInterface,
	scopeService DashboardScopeServiceInterface,
	userDao daos.UserDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
) DashboardServiceInterface {
	return &dashboardService{
		dashboardDao: dashboardDao,
		scopeService: scopeService,
		userDao:      userDao,
		groupUserDao: groupUserDao,
	}
}

// ---------- filtros ----------

// Filters devuelve equipos, grupos, corredores, tipos de ejercicio, catálogo y
// rango de fechas REAL, todo recortado por el scope del solicitante.
func (s *dashboardService) Filters(ctx *gin.Context, callerID, teamID int64) (*dashboard.FiltersResponse, error) {
	scope, err := s.scopeService.Resolve(ctx, callerID)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.resolveTeamGroup(ctx, scope, &teamID, nil)
	if err != nil {
		return nil, err
	}

	response := &dashboard.FiltersResponse{
		Teams:         []dashboard.FiltersTeam{},
		Groups:        []dashboard.FiltersGroup{},
		Athletes:      []dashboard.FiltersAthlete{},
		ExerciseTypes: constants.CurrentExerciseKinds(),
		Exercises:     []dashboard.FiltersExercise{},
	}

	// El scope del request es el del equipo del que se llegó: aunque el corredor
	// pertenezca a otros equipos, /filters de este equipo no puede ofrecer los de
	// otro — el equipo es el ancla de la autorización.
	if _, inScope := scope.Teams[teamID]; !inScope {
		return nil, ErrDashboardForbidden
	}
	response.Teams = append(response.Teams, dashboard.FiltersTeam{ID: teamID, Name: scope.Teams[teamID].Name})

	_, ownsTeam := scope.OwnedTeamIDs[teamID]
	for _, groupID := range groupIDs {
		group := scope.Groups[groupID]
		role := "member"
		if _, isOwnerGroup := scope.OwnedGroupIDs[groupID]; isOwnerGroup {
			role = "owner"
		}
		response.Groups = append(response.Groups, dashboard.FiltersGroup{
			ID: group.ID, TeamID: group.TeamID, Name: group.Name, Role: role,
		})
	}

	athleteIDs, err := s.dashboardDao.ScopeAthletes(ctx, groupIDs)
	if err != nil {
		customlogger.Error(ctx, "error listing dashboard scope athletes", err,
			customlogger.TagMethod("Dashboard.Filters"))
		return nil, fmt.Errorf("error al resolver los corredores del dashboard")
	}
	// El trainer necesita el roster completo (leaderboard y comparaciones lo
	// requieren); el corredor ve solo su propia entrada, aunque comparta grupo.
	if !ownsTeam {
		athleteIDs = intersectIDs(athleteIDs, []int64{callerID})
	}
	names, err := s.athleteNames(ctx, athleteIDs)
	if err != nil {
		return nil, err
	}
	for _, athleteID := range athleteIDs {
		name, ok := names[athleteID]
		if !ok {
			continue
		}
		response.Athletes = append(response.Athletes, dashboard.FiltersAthlete{
			ID: athleteID, Name: name.Name, Surname: name.Surname,
		})
	}

	exercises, err := s.catalogExercisesOfTeamOwners(ctx, scope, teamID)
	if err != nil {
		return nil, err
	}
	response.Exercises = exercises

	minDate, maxDate, err := s.dashboardDao.ScopeDateRange(ctx, groupIDs)
	if err != nil {
		customlogger.Error(ctx, "error reading dashboard date range", err,
			customlogger.TagMethod("Dashboard.Filters"))
		return nil, fmt.Errorf("error al resolver el rango de fechas del dashboard")
	}
	// Null y no {min: null, max: null}: el front usa ese null para distinguir
	// "nunca hubo datos" de "hay datos acotados".
	if minDate != nil && maxDate != nil {
		response.DateRange = &dashboard.DateRange{
			Min: dashboard.FormatDate(*minDate),
			Max: dashboard.FormatDate(*maxDate),
		}
	}
	return response, nil
}

// ---------- métricas ----------

func (s *dashboardService) Summary(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.SummaryResponse, error) {
	prepared, err := s.prepare(ctx, callerID, query, false)
	if err != nil {
		return nil, err
	}
	rows, openSessions, err := s.seriesAndQuality(ctx, prepared)
	if err != nil {
		return nil, err
	}
	response := dashboard.BuildSummary(rows, prepared.Tolerance, openSessions)
	return &response, nil
}

func (s *dashboardService) TimeSeries(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.TimeseriesResponse, error) {
	prepared, err := s.prepare(ctx, callerID, query, false)
	if err != nil {
		return nil, err
	}
	rows, openSessions, err := s.seriesAndQuality(ctx, prepared)
	if err != nil {
		return nil, err
	}
	response := dashboard.BuildTimeSeries(rows, prepared.Granularity, prepared.Tolerance, openSessions)
	return &response, nil
}

func (s *dashboardService) Leaderboard(ctx *gin.Context, callerID int64, query MetricsQuery) (*dashboard.LeaderboardResponse, error) {
	prepared, err := s.prepare(ctx, callerID, query, true)
	if err != nil {
		return nil, err
	}
	rows, _, err := s.seriesAndQuality(ctx, prepared)
	if err != nil {
		return nil, err
	}
	openSessions, err := s.countOpenSessions(ctx, prepared)
	if err != nil {
		return nil, err
	}

	byAthlete := map[int64][]dashboard.ScopedSeries{}
	for _, row := range rows {
		byAthlete[row.AthleteUserID] = append(byAthlete[row.AthleteUserID], row)
	}
	athleteIDs := make([]int64, 0, len(byAthlete))
	for athleteID := range byAthlete {
		athleteIDs = append(athleteIDs, athleteID)
	}
	sort.Slice(athleteIDs, func(i, j int) bool { return athleteIDs[i] < athleteIDs[j] })

	names, err := s.athleteNames(ctx, athleteIDs)
	if err != nil {
		return nil, err
	}
	entries := make([]dashboard.LeaderboardEntry, 0, len(athleteIDs))
	for _, athleteID := range athleteIDs {
		name := names[athleteID]
		entries = append(entries, dashboard.NewLeaderboardEntry(athleteID, name.Name, name.Surname,
			byAthlete[athleteID], prepared.Tolerance))
	}

	quality := dashboard.BuildSummary(rows, prepared.Tolerance, openSessions).DataQuality
	response := dashboard.BuildLeaderboard(entries, prepared.Sort, prepared.Order,
		prepared.Page, prepared.PageSize, callerID, quality)
	return &response, nil
}

// ExerciseSeries devuelve el detalle por serie de un ejercicio de CATÁLOGO. El id
// del path es exercises.id, el mismo que expone /filters, para que el selector de
// la barra de filtros pueda encadenarse directo.
func (s *dashboardService) ExerciseSeries(ctx *gin.Context, callerID, catalogExerciseID int64, query MetricsQuery) (*dashboard.SeriesResponse, error) {
	prepared, err := s.prepare(ctx, callerID, query, false)
	if err != nil {
		return nil, err
	}
	// El filtro por ejercicio del request tiene que ser el del path, o el
	// endpoint podría devolver series de otro ejercicio.
	prepared.ExerciseIDs = []int64{catalogExerciseID}
	prepared.LegacyExerciseInstanceIDs, err = s.resolveLegacyInstances(ctx, prepared, catalogExerciseID)
	if err != nil {
		return nil, err
	}

	rows, openSessions, err := s.seriesAndQuality(ctx, prepared)
	if err != nil {
		return nil, err
	}
	quality := dashboard.BuildSummary(rows, prepared.Tolerance, openSessions).DataQuality
	response := dashboard.BuildSeriesDetail(rows, prepared.Tolerance, prepared.Page, prepared.PageSize, quality)
	return &response, nil
}

// ---------- comparaciones ----------

// Comparisons compara corredores del scope. Cada athlete_user_id se valida por
// separado: si uno está fuera de scope responde 403 nombrándolo, sin devolver los
// datos de los válidos (spec "Comparación con un corredor fuera de scope").
func (s *dashboardService) Comparisons(ctx *gin.Context, callerID int64, request dashboard.ComparisonsRequest) (*dashboard.ComparisonsResponse, error) {
	scope, err := s.scopeService.Resolve(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if !scope.OwnsTeam(request.TeamID) {
		return nil, ErrDashboardForbidden
	}
	query, err := s.validate(ctx, scope, MetricsQuery{
		TeamID:         request.TeamID,
		GroupID:        request.GroupID,
		ExerciseID:     request.ExerciseID,
		ExerciseIDs:    request.ExerciseIDs,
		Kinds:          request.Kinds,
		From:           request.From,
		To:             request.To,
		Tolerance:      formatOptionalFloat(request.Tolerance),
		AthleteUserIDs: request.AthleteUserIDs,
	}, true)
	if err != nil {
		return nil, err
	}
	if err := s.validateAthletes(ctx, scope, query.GroupIDs, query.AthleteUserIDs); err != nil {
		return nil, err
	}
	if len(query.AthleteUserIDs) == 0 {
		query.AthleteUserIDs, err = s.dashboardDao.ScopeAthletes(ctx, query.GroupIDs)
		if err != nil {
			customlogger.Error(ctx, "error listing dashboard comparison athletes", err,
				customlogger.TagMethod("Dashboard.Comparisons"))
			return nil, fmt.Errorf("error al resolver los corredores del dashboard")
		}
	}

	rows, openSessions, err := s.seriesAndQuality(ctx, query)
	if err != nil {
		return nil, err
	}
	quality := dashboard.BuildSummary(rows, query.Tolerance, openSessions).DataQuality
	response := dashboard.BuildComparisons(rows, query.Tolerance, quality)
	names, err := s.athleteNames(ctx, query.AthleteUserIDs)
	if err != nil {
		return nil, err
	}
	dashboard.ApplyAthleteNames(response.Series, names)
	return &response, nil
}

// Periods compara el período pedido contra el inmediatamente anterior de la misma
// longitud. previous es opcional: sin él, el backend lo deriva.
func (s *dashboardService) Periods(ctx *gin.Context, callerID int64, request dashboard.PeriodsRequest) (*dashboard.PeriodsResponse, error) {
	scope, err := s.scopeService.Resolve(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if !scope.OwnsTeam(request.TeamID) {
		return nil, ErrDashboardForbidden
	}
	base, err := s.validate(ctx, scope, MetricsQuery{
		TeamID:      request.TeamID,
		GroupID:     request.GroupID,
		ExerciseID:  request.ExerciseID,
		ExerciseIDs: request.ExerciseIDs,
		Kinds:       request.Kinds,
		Tolerance:   formatOptionalFloat(request.Tolerance),
	}, false)
	if err != nil {
		return nil, err
	}

	currentFrom, currentTo, err := parseDashboardRange(request.Current.From, request.Current.To)
	if err != nil {
		return nil, err
	}
	previousFrom, previousTo := dashboard.DerivePreviousPeriod(currentFrom, currentTo)
	if request.Previous != nil {
		previousFrom, previousTo, err = parseDashboardRange(request.Previous.From, request.Previous.To)
		if err != nil {
			return nil, err
		}
	}

	currentQuery := *base
	currentQuery.From, currentQuery.To = currentFrom, currentTo
	currentRows, _, err := s.seriesAndQuality(ctx, &currentQuery)
	if err != nil {
		return nil, err
	}
	previousQuery := *base
	previousQuery.From, previousQuery.To = previousFrom, previousTo
	previousRows, err := s.fetchSeries(ctx, &previousQuery)
	if err != nil {
		return nil, err
	}

	// El data_quality de la respuesta es el del período actual: es el que la
	// pantalla muestra junto a los KPIs que está mirando.
	openSessions, err := s.countOpenSessions(ctx, &currentQuery)
	if err != nil {
		return nil, err
	}
	quality := dashboard.BuildSummary(currentRows, base.Tolerance, openSessions).DataQuality
	response := dashboard.BuildPeriods(currentFrom, currentTo, previousFrom, previousTo,
		currentRows, previousRows, base.Tolerance, quality)
	return &response, nil
}

// ---------- validación ----------

// prepare es el camino común: valida los parámetros y resuelve el scope. Con
// ownerOnly se aplica el 403 de las tres endpoints de comparación (la regla vive
// en el backend y no en el front).
func (s *dashboardService) prepare(ctx *gin.Context, callerID int64, query MetricsQuery, ownerOnly bool) (*validatedMetricsQuery, error) {
	scope, err := s.scopeService.Resolve(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if ownerOnly && !scope.OwnsTeam(query.TeamID) {
		return nil, ErrDashboardForbidden
	}
	return s.validate(ctx, scope, query, true)
}

// validate aplica la whitelist y las cotas ANTES de tocar la DB (spec "Los
// parámetros inválidos se rechazan con 400 y sin consulta").
//
// requireRange es false en /comparisons/periods, que valida sus dos períodos por
// su cuenta: el `from`/`to` de la query compartida no aplica ahí.
func (s *dashboardService) validate(ctx *gin.Context, scope *DashboardScope, query MetricsQuery, requireRange bool) (*validatedMetricsQuery, error) {
	groupIDs, err := s.resolveTeamGroup(ctx, scope, &query.TeamID, query.GroupID)
	if err != nil {
		return nil, err
	}

	var from, to time.Time
	if requireRange {
		from, to, err = parseDashboardRange(query.From, query.To)
		if err != nil {
			return nil, err
		}
		if span := int(to.Sub(from).Hours() / 24); span > dashboardMaxRangeDays {
			return nil, newDashboardParamError(constants.ErrorCodeInvalidRange,
				"el rango no puede superar los 2 años (va de %s a %s)",
				dashboard.FormatDate(from), dashboard.FormatDate(to))
		}
	}

	tolerance, err := parseDashboardTolerance(query.Tolerance)
	if err != nil {
		return nil, err
	}
	granularity, err := parseDashboardGranularity(query.Granularity)
	if err != nil {
		return nil, err
	}
	sortColumn, err := parseDashboardSort(query.Sort)
	if err != nil {
		return nil, err
	}
	order, err := parseDashboardOrder(query.Order)
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parseDashboardPaging(query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}

	exerciseIDs, kinds, err := s.resolveExerciseFilter(ctx, scope, query)
	if err != nil {
		return nil, err
	}

	return &validatedMetricsQuery{
		Scope:          scope,
		CallerID:       scope.CallerID,
		TeamID:         query.TeamID,
		GroupIDs:       groupIDs,
		ExerciseIDs:    exerciseIDs,
		Kinds:          kinds,
		From:           from,
		To:             to,
		Granularity:    granularity,
		Tolerance:      tolerance,
		Page:           page,
		PageSize:       pageSize,
		Sort:           sortColumn,
		Order:          order,
		AthleteUserIDs: query.AthleteUserIDs,
	}, nil
}

// parseDashboardRange aplica el contrato de fechas: from obligatorio, ISO de
// entrada y salida, y to opcional (sin él, el rango cierra en from).
func parseDashboardRange(fromRaw, toRaw string) (time.Time, time.Time, error) {
	if strings.TrimSpace(fromRaw) == "" {
		return time.Time{}, time.Time{}, newDashboardParamError(constants.ErrorCodeMissingFrom,
			"from es obligatorio")
	}
	from, err := time.Parse(dashboard.DateLayoutISO, fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, newDashboardParamError(constants.ErrorCodeInvalidRange,
			"from debe tener formato YYYY-MM-DD")
	}
	to := from
	if strings.TrimSpace(toRaw) != "" {
		to, err = time.Parse(dashboard.DateLayoutISO, toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, newDashboardParamError(constants.ErrorCodeInvalidRange,
				"to debe tener formato YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, newDashboardParamError(constants.ErrorCodeInvalidRange,
			"to no puede ser anterior a from")
	}
	return dateOnly(from), dateOnly(to), nil
}

func parseDashboardTolerance(raw string) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return dashboard.DefaultTolerance, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < dashboardMinTolerance || value > dashboardMaxTolerance {
		return 0, newDashboardParamError(constants.ErrorCodeInvalidTolerance,
			"tolerance debe estar entre %s y %s", trimFloat(dashboardMinTolerance), trimFloat(dashboardMaxTolerance))
	}
	return value, nil
}

func parseDashboardGranularity(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return dashboardDefaultGranularity, nil
	}
	if !dashboardGranularities[raw] {
		return "", newDashboardParamError(constants.ErrorCodeInvalidGranularity,
			"granularity debe ser uno de: session, week, month, quarter")
	}
	return raw, nil
}

func parseDashboardSort(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return dashboardDefaultSort, nil
	}
	if !dashboard.DashboardSortWhitelist[raw] {
		return "", newDashboardParamError(constants.ErrorCodeInvalidSort,
			"sort debe ser uno de: best_time_ms, mean_speed_kph, total_distance_m, compliance_rate")
	}
	return raw, nil
}

func parseDashboardOrder(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return dashboardDefaultOrder, nil
	}
	if !dashboardOrders[raw] {
		// El conjunto de codes del spec es cerrado y no tiene uno para `order`:
		// `order` es parte de la misma cláusula de orden que `sort`, así que comparte
		// su código en vez de inventar uno nuevo que el cliente no conoce.
		return "", newDashboardParamError(constants.ErrorCodeInvalidSort,
			"order debe ser asc o desc")
	}
	return raw, nil
}

func parseDashboardPaging(pageRaw, pageSizeRaw string) (int, int, error) {
	page := dashboardDefaultPage
	if strings.TrimSpace(pageRaw) != "" {
		parsed, err := strconv.Atoi(pageRaw)
		if err != nil || parsed < 1 {
			return 0, 0, newDashboardParamError(constants.ErrorCodeInvalidPageSize,
				"page debe ser un entero mayor o igual a 1")
		}
		page = parsed
	}
	pageSize := dashboardDefaultPageSize
	if strings.TrimSpace(pageSizeRaw) != "" {
		parsed, err := strconv.Atoi(pageSizeRaw)
		if err != nil || parsed < 1 || parsed > dashboardMaxPageSize {
			return 0, 0, newDashboardParamError(constants.ErrorCodeInvalidPageSize,
				"page_size debe estar entre 1 y %d", dashboardMaxPageSize)
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

// ---------- resolución de filtros ----------

// resolveExerciseFilter aplica las tres formas excluyentes de elegir ejercicio
// (spec "Los endpoints de métricas declaran sus filtros"): exactamente una de
// exercise_id, exercise_ids[] o kinds[], o ninguna.
//
// kinds[] se resuelve a los ids de catálogo de esos tipos YA en el backend: el
// front tiene prohibido recalcular KPIs, así que un ritmo_continuo de 4 km y uno
// de 21 km llegan en la misma fila con sus KPIs agregados por volumen.
func (s *dashboardService) resolveExerciseFilter(ctx *gin.Context, scope *DashboardScope, query MetricsQuery) ([]int64, []string, error) {
	selected := 0
	if query.ExerciseID != nil {
		selected++
	}
	if len(query.ExerciseIDs) > 0 {
		selected++
	}
	if len(query.Kinds) > 0 {
		selected++
	}
	if selected > 1 {
		return nil, nil, newDashboardParamError(constants.ErrorCodeInvalidRange,
			"exercise_id, exercise_ids y kinds son excluyentes: mandá solo uno")
	}

	switch {
	case query.ExerciseID != nil:
		return s.catalogExercisesInScope(ctx, scope, query.TeamID, []int64{*query.ExerciseID}, nil)
	case len(query.ExerciseIDs) > 0:
		return s.catalogExercisesInScope(ctx, scope, query.TeamID, query.ExerciseIDs, nil)
	case len(query.Kinds) > 0:
		for _, kind := range query.Kinds {
			if !constants.IsCurrentExerciseKind(kind) {
				return nil, nil, newDashboardParamError(constants.ErrorCodeInvalidRange,
					"kind %q no es un tipo vigente", kind)
			}
		}
		return s.catalogExercisesInScope(ctx, scope, query.TeamID, nil, query.Kinds)
	default:
		return nil, nil, nil
	}
}

// catalogExercisesInScope traduce el filtro de ejercicio a ids de catálogo de los
// dueños del equipo pedido, y se asegura de que esos ejercicios existan. Un
// ejercicio de otro entrenador no está en el scope: 404 si no existe, 403 si
// existe pero es de otro dueño.
func (s *dashboardService) catalogExercisesInScope(ctx *gin.Context, scope *DashboardScope, teamID int64, ids []int64, kinds []string) ([]int64, []string, error) {
	ownerIDs := s.teamOwnerIDs(scope, teamID)
	resolved := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, ownerID := range ownerIDs {
		var found []daos.DashboardCatalogExercises
		var err error
		if len(kinds) > 0 {
			// Los tipos del filtro, no la lista entera de vigentes.
			found, err = s.dashboardDao.FindCatalogExercisesByOwner(ctx, ownerID, kinds)
		} else {
			found, err = s.dashboardDao.FindDashboardCatalogExercisesByIDs(ctx, ids)
		}
		if err != nil {
			customlogger.Error(ctx, "error resolving dashboard exercise filter", err,
				customlogger.TagMethod("Dashboard.resolveExerciseFilter"))
			return nil, nil, fmt.Errorf("error al resolver el filtro de ejercicios")
		}
		for _, exercise := range found {
			if exercise.OwnerID != ownerID {
				continue
			}
			if _, dup := seen[exercise.ID]; dup {
				continue
			}
			seen[exercise.ID] = struct{}{}
			resolved = append(resolved, exercise.ID)
		}
	}
	if len(kinds) > 0 {
		return resolved, kinds, nil
	}
	if len(resolved) == 0 {
		return nil, nil, ErrDashboardNotFound
	}
	if len(resolved) != len(dedupeIDs(ids)) {
		return nil, nil, ErrDashboardNotFound
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i] < resolved[j] })
	return resolved, nil, nil
}

// resolveLegacyInstances traduce las ExerciseInstance sin source_exercise_id a
// ids de catálogo por coincidencia de nombre normalizado, acotada al mismo dueño.
//
// Descartarlas perdería ese histórico EN SILENCIO, que es el riesgo principal de
// /exercises/{id}/series. Un nombre que matchea más de un ejercicio del mismo
// dueño resuelve al id más bajo (dashboard.NewNameIndex) y se loguea: el fallback
// tiene que ser visible, no una decisión invisible de una query.
func (s *dashboardService) resolveLegacyInstances(ctx *gin.Context, prepared *validatedMetricsQuery, catalogExerciseID int64) ([]int64, error) {
	catalog, err := s.dashboardDao.FindDashboardCatalogExerciseByID(ctx, catalogExerciseID)
	if err != nil {
		customlogger.Error(ctx, "error reading dashboard catalog exercise", err,
			customlogger.TagMethod("Dashboard.resolveLegacyInstances"))
		return nil, fmt.Errorf("error al resolver el ejercicio del dashboard")
	}
	if catalog == nil {
		return nil, ErrDashboardNotFound
	}
	if !catalogOwnerInScope(prepared.Scope, prepared.TeamID, catalog.OwnerID) {
		return nil, ErrDashboardForbidden
	}

	unlinked, err := s.dashboardDao.UnlinkedExerciseInstances(ctx, prepared.GroupIDs)
	if err != nil {
		customlogger.Error(ctx, "error listing unlinked exercise instances", err,
			customlogger.TagMethod("Dashboard.resolveLegacyInstances"))
		return nil, fmt.Errorf("error al resolver el histórico del ejercicio")
	}
	if len(unlinked) == 0 {
		return nil, nil
	}

	index := dashboard.NewNameIndex([]dashboard.CatalogExercise{{
		ID: catalog.ID, OwnerID: catalog.OwnerID, Name: catalog.Name,
	}})
	ambiguous := map[string]int{}
	resolved := make([]int64, 0, len(unlinked))
	for _, instance := range unlinked {
		catalogID, isAmbiguous, ok := index.Resolve(instance.OwnerID, instance.Name)
		if !ok || catalogID != catalog.ID {
			continue
		}
		resolved = append(resolved, instance.ExerciseInstanceID)
		if isAmbiguous {
			ambiguous[dashboard.NormalizeExerciseName(instance.Name)]++
		}
	}
	if len(ambiguous) > 0 {
		customlogger.Warn(ctx, "dashboard legacy name fallback matched several catalog exercises",
			customlogger.Tag("exercise_id", strconv.FormatInt(catalog.ID, 10)),
			customlogger.Tag("owner_id", strconv.FormatInt(catalog.OwnerID, 10)),
			customlogger.Tag("ambiguous_names", strconv.Itoa(len(ambiguous))),
			customlogger.Tag("resolved_instances", strconv.Itoa(len(resolved))))
	}
	return resolved, nil
}

// ---------- consultas ----------

func (s *dashboardService) fetchSeries(ctx *gin.Context, prepared *validatedMetricsQuery) ([]dashboard.ScopedSeries, error) {
	rows, err := s.dashboardDao.FindSeries(ctx, daos.DashboardScopeFilter{
		GroupIDs:                  prepared.GroupIDs,
		From:                      prepared.From,
		To:                        prepared.To,
		Granularity:               prepared.Granularity,
		ExerciseIDs:               prepared.ExerciseIDs,
		LegacyExerciseInstanceIDs: prepared.LegacyExerciseInstanceIDs,
		AthleteUserIDs:            prepared.AthleteUserIDs,
	})
	if err != nil {
		customlogger.Error(ctx, "error finding dashboard series", err,
			customlogger.TagMethod("Dashboard.fetchSeries"))
		return nil, fmt.Errorf("error al obtener las series del dashboard")
	}
	return mapSeriesRows(rows), nil
}

func (s *dashboardService) countOpenSessions(ctx *gin.Context, prepared *validatedMetricsQuery) (int, error) {
	open, err := s.dashboardDao.CountOpenSessions(ctx, daos.DashboardScopeFilter{
		GroupIDs:       prepared.GroupIDs,
		From:           prepared.From,
		To:             prepared.To,
		AthleteUserIDs: prepared.AthleteUserIDs,
	})
	if err != nil {
		customlogger.Error(ctx, "error counting dashboard open sessions", err,
			customlogger.TagMethod("Dashboard.countOpenSessions"))
		return 0, fmt.Errorf("error al contar las sesiones abiertas del dashboard")
	}
	return open.Sessions, nil
}

func (s *dashboardService) seriesAndQuality(ctx *gin.Context, prepared *validatedMetricsQuery) ([]dashboard.ScopedSeries, int, error) {
	rows, err := s.fetchSeries(ctx, prepared)
	if err != nil {
		return nil, 0, err
	}
	openSessions, err := s.countOpenSessions(ctx, prepared)
	if err != nil {
		return nil, 0, err
	}
	return rows, openSessions, nil
}

// ---------- helpers ----------

func mapSeriesRows(rows []daos.DashboardSeriesRow) []dashboard.ScopedSeries {
	out := make([]dashboard.ScopedSeries, 0, len(rows))
	for _, row := range rows {
		out = append(out, dashboard.ScopedSeries{
			Series: dashboard.Series{
				Kind:             row.Kind,
				DurationMs:       row.DurationMs,
				ActiveDurationMs: row.ActiveDurationMs,
				DistanceMeters:   row.DistanceMeters,
				CompletionStatus: row.CompletionStatus,
				HasRunnerSession: row.HasRunnerSession,
			},
			FeedbackID:          row.FeedbackID,
			SessionInstanceID:   row.SessionInstanceID,
			ExerciseInstanceID:  row.ExerciseInstanceID,
			SessionDate:         row.SessionDate,
			BucketStart:         row.BucketStart,
			AthleteUserID:       row.AthleteUserID,
			FeedbackOwnerUserID: row.FeedbackOwnerUserID,
			SetNumber:           row.SetNumber,
			TargetMinutes:       row.TargetMinutes,
			TargetDistanceM:     row.TargetDistanceM,
		})
	}
	return out
}

func (s *dashboardService) athleteNames(ctx *gin.Context, athleteIDs []int64) (map[int64]dashboard.AthleteName, error) {
	names := map[int64]dashboard.AthleteName{}
	if len(athleteIDs) == 0 {
		return names, nil
	}
	users, err := s.userDao.FindByIDs(ctx, athleteIDs)
	if err != nil {
		customlogger.Error(ctx, "error resolving dashboard athlete names", err,
			customlogger.TagMethod("Dashboard.athleteNames"))
		return nil, fmt.Errorf("error al resolver los corredores del dashboard")
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		names[user.ID] = dashboard.AthleteName{Name: user.Name, Surname: user.Surname}
	}
	return names, nil
}

// catalogExercisesOfTeamOwners arma el catálogo que ve /filters: los ejercicios
// de los dueños de los grupos del scope. Los owners de un equipo son uno solo
// (teams.owner_id), así que en la práctica es un dueño por equipo.
func (s *dashboardService) catalogExercisesOfTeamOwners(ctx *gin.Context, scope *DashboardScope, teamID int64) ([]dashboard.FiltersExercise, error) {
	exercises := []dashboard.FiltersExercise{}
	seen := map[int64]struct{}{}
	for _, ownerID := range s.teamOwnerIDs(scope, teamID) {
		found, err := s.dashboardDao.FindCatalogExercisesByOwner(ctx, ownerID, constants.CurrentExerciseKinds())
		if err != nil {
			customlogger.Error(ctx, "error listing dashboard catalog exercises", err,
				customlogger.TagMethod("Dashboard.Filters"))
			return nil, fmt.Errorf("error al resolver el catálogo del dashboard")
		}
		for _, exercise := range found {
			if _, dup := seen[exercise.ID]; dup {
				continue
			}
			seen[exercise.ID] = struct{}{}
			exercises = append(exercises, dashboard.FiltersExercise{
				ID:                     exercise.ID,
				Name:                   exercise.Name,
				Kind:                   exercise.Kind,
				CurrentTargetMinutes:   exercise.Minutes,
				CurrentTargetDistanceM: exercise.DistanceM,
			})
		}
	}
	return exercises, nil
}

func (s *dashboardService) teamOwnerIDs(scope *DashboardScope, teamID int64) []int64 {
	team, ok := scope.Teams[teamID]
	if !ok {
		return nil
	}
	return []int64{team.OwnerID}
}

// catalogOwnerInScope acota un ejercicio de catálogo al dueño del equipo pedido:
// `exercises` es catálogo por dueño, así que un ejercicio de otro entrenador no
// puede entrar ni por id.
func catalogOwnerInScope(scope *DashboardScope, teamID, ownerID int64) bool {
	team, ok := scope.Teams[teamID]
	return ok && team.OwnerID == ownerID
}

// validateAthletes rechaza, uno por uno, los corredores fuera de scope.
func (s *dashboardService) validateAthletes(ctx *gin.Context, scope *DashboardScope, groupIDs, athleteUserIDs []int64) error {
	if len(athleteUserIDs) == 0 {
		return nil
	}
	allowed := map[int64]struct{}{}
	for _, groupID := range groupIDs {
		members, err := s.groupUserDao.FindByGroupID(ctx, groupID)
		if err != nil {
			customlogger.Error(ctx, "error validating dashboard athletes", err,
				customlogger.TagMethod("Dashboard.validateAthletes"))
			return fmt.Errorf("error al validar los corredores del dashboard")
		}
		for _, member := range members {
			allowed[member.UserID] = struct{}{}
		}
	}
	for _, athleteID := range athleteUserIDs {
		if _, ok := allowed[athleteID]; !ok {
			return newDashboardAthleteForbidden(athleteID)
		}
	}
	return nil
}

func (s *dashboardService) resolveTeamGroup(ctx *gin.Context, scope *DashboardScope, teamID, groupID *int64) ([]int64, error) {
	if teamID == nil {
		return nil, ErrDashboardNotFound
	}
	if _, inScope := scope.Teams[*teamID]; !inScope {
		return nil, ErrDashboardForbidden
	}
	groupIDs := scope.GroupsForTeam(*teamID)
	if groupID == nil {
		return groupIDs, nil
	}
	group, err := s.scopeService.FindGroup(ctx, *groupID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar grupo")
	}
	if group == nil {
		return nil, ErrDashboardNotFound
	}
	if !scope.CanReadGroup(*groupID) {
		return nil, ErrDashboardForbidden
	}
	return []int64{*groupID}, nil
}

func dateOnly(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}

func intersectIDs(all, wanted []int64) []int64 {
	keep := map[int64]struct{}{}
	for _, id := range wanted {
		keep[id] = struct{}{}
	}
	out := make([]int64, 0, len(wanted))
	for _, id := range all {
		if _, ok := keep[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func dedupeIDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
