package services

// Competencias (gamificacion-core E4.1, D8/D9). CRUD + moderación + cierre:
// E4.2 computa winners al cerrar (metric por ventana; route con matching D10),
// stamps a los ganadores, unseen por participante y push best-effort a todos.

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
	"simple-arq-golang/cmd/api/utils/gametime"
	"simple-arq-golang/cmd/api/utils/routematch"
)

var (
	ErrCompetitionInvalid   = errors.New("datos de competencia inválidos")
	ErrCompetitionForbidden = errors.New("no tenés permisos para operar sobre esta competencia")
	ErrCompetitionConflict  = errors.New("transición de estado inválida")
)

// métricas válidas para type=metric: las mismas fuente de feedback (D6/D7).
var validCompetitionMetrics = []string{"distance", "duration", "pace", "peak_speed"}

// límites de rutas y radius (spec 4b, D4/D5).
const (
	maxRouteVertices     = 500
	maxRouteLengthMeters = 100000.0
	minAcceptanceRadiusM = 5.0
	maxAcceptanceRadiusM = 500.0
)

type CompetitionServiceInterface interface {
	Create(ctx *gin.Context, callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error)
	Approve(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error)
	Reject(ctx *gin.Context, callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error)
	Cancel(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error)
	ListByTeam(ctx *gin.Context, callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error)
	Get(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionDetailResponse, error)
	// Close aplica el cierre (status finished + efectos): devuelve la respuesta
	// y los session_instance_ids de los intentos registrados de la competencia
	// para el control:competition_finished del controller (D9); sin intentos
	// = slice vacío.
	Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error)
}

type competitionService struct {
	dao          daos.CompetitionDAOInterface
	pushTokenDao daos.PushTokenDaoInterface
	pushClient   expopushclient.ExpoPushClientInterface
}

func NewCompetitionService(dao daos.CompetitionDAOInterface, pushTokenDao daos.PushTokenDaoInterface, pushClient expopushclient.ExpoPushClientInterface) CompetitionServiceInterface {
	return &competitionService{dao: dao, pushTokenDao: pushTokenDao, pushClient: pushClient}
}

func (s *competitionService) Create(ctx *gin.Context, callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}
	if req.GroupID != nil {
		belongs, err := s.dao.GroupBelongsToTeam(ctx, *req.GroupID, teamID)
		if err != nil {
			return nil, fmt.Errorf("error al crear la competencia")
		}
		if !belongs {
			return nil, fmt.Errorf("%w: group_id no pertenece al equipo", ErrCompetitionInvalid)
		}
	}

	// Guard: sin ser owner ni miembro el DAO de competencias no se toca (403).
	isOwner, err := s.dao.IsTeamOwner(ctx, teamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al crear la competencia")
	}
	status := competition.StatusPendingApproval
	role := competition.CreatorRoleRunner
	if isOwner {
		status = competition.StatusActive
		role = competition.CreatorRoleTrainer
	} else {
		isMember, err := s.dao.IsTeamGroupMember(ctx, teamID, callerID)
		if err != nil {
			return nil, fmt.Errorf("error al crear la competencia")
		}
		if !isMember {
			return nil, ErrCompetitionForbidden
		}
	}

	// Participantes: únicos, >= 2, todos miembros del team, sin el trainer.
	seen := make(map[int64]bool, len(req.ParticipantIDs))
	unique := make([]int64, 0, len(req.ParticipantIDs))
	for _, id := range req.ParticipantIDs {
		if id <= 0 || seen[id] {
			return nil, fmt.Errorf("%w: participants debe ir sin duplicados ni ids negativos", ErrCompetitionInvalid)
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) < 2 {
		return nil, fmt.Errorf("%w: al menos 2 participantes distintos", ErrCompetitionInvalid)
	}
	memberCount, err := s.dao.CountTeamGroupMembers(ctx, teamID, unique)
	if err != nil {
		return nil, fmt.Errorf("error al crear la competencia")
	}
	if int(memberCount) != len(unique) {
		return nil, fmt.Errorf("%w: todos los participantes deben ser miembros activos del equipo", ErrCompetitionInvalid)
	}

	if isOwner {
		for _, id := range unique {
			if id == callerID {
				return nil, fmt.Errorf("%w: el entrenador no compite (no puede estar entre los participantes)", ErrCompetitionInvalid)
			}
		}
	}

	comp := &dbs.Competition{
		TeamID:                 teamID,
		GroupID:                req.GroupID,
		Type:                   req.Type,
		Metric:                 req.Metric,
		StartAt:                req.StartAt,
		EndAt:                  req.EndAt,
		RouteGeoJSON:           req.RouteGeoJSON,
		AcceptanceRadiusMeters: req.AcceptanceRadiusMeters,
		CreatedByUserID:        callerID,
		CreatedByRole:          role,
		Status:                 status,
	}
	if err := s.dao.Create(ctx, comp, unique); err != nil {
		customlogger.Error(ctx, "error creating competition", err, customlogger.TagMethod("CompetitionCreate"))
		return nil, fmt.Errorf("error al crear la competencia")
	}
	return toCompetitionResponse(comp, unique, nil), nil
}

// validateCreateRequest aplica las reglas cruzadas del spec: metric exige
// metric + end_at > start_at; route exige route_geojson (parseable, ≤500
// vértices, ≤100 km) + radius en [5, 500].
func validateCreateRequest(req competition.CreateCompetitionRequest) error {
	if req.Type != competition.TypeMetric && req.Type != competition.TypeRoute {
		return fmt.Errorf("%w: type debe ser metric o route", ErrCompetitionInvalid)
	}
	if req.StartAt.IsZero() {
		return fmt.Errorf("%w: start_at es obligatorio", ErrCompetitionInvalid)
	}
	switch req.Type {
	case competition.TypeMetric:
		if req.Metric == nil || *req.Metric == "" {
			return fmt.Errorf("%w: type=metric exige metric", ErrCompetitionInvalid)
		}
		validMetric := false
		for _, m := range validCompetitionMetrics {
			if m == *req.Metric {
				validMetric = true
			}
		}
		if !validMetric {
			return fmt.Errorf("%w: metric debe ser una de: distance, duration, pace, peak_speed", ErrCompetitionInvalid)
		}
		if req.EndAt == nil {
			return fmt.Errorf("%w: type=metric exige end_at posterior a start_at", ErrCompetitionInvalid)
		}
		if !req.EndAt.After(req.StartAt) {
			return fmt.Errorf("%w: end_at debe ser posterior a start_at", ErrCompetitionInvalid)
		}
		if req.RouteGeoJSON != nil || req.AcceptanceRadiusMeters != nil {
			return fmt.Errorf("%w: type=metric no admite route_geojson ni acceptance_radius_meters", ErrCompetitionInvalid)
		}
	case competition.TypeRoute:
		if req.RouteGeoJSON == nil || *req.RouteGeoJSON == "" {
			return fmt.Errorf("%w: type=route exige route_geojson", ErrCompetitionInvalid)
		}
		vertices, err := routematch.ParseLineString(*req.RouteGeoJSON)
		if err != nil {
			return fmt.Errorf("%w: la ruta GeoJSON no es válida", ErrCompetitionInvalid)
		}
		if len(vertices) > maxRouteVertices {
			return fmt.Errorf("%w: la ruta admite hasta 500 puntos", ErrCompetitionInvalid)
		}
		if routematch.LineStringLength(vertices) > maxRouteLengthMeters {
			return fmt.Errorf("%w: la ruta no puede superar los 100 km", ErrCompetitionInvalid)
		}
		if req.AcceptanceRadiusMeters == nil ||
			*req.AcceptanceRadiusMeters < minAcceptanceRadiusM ||
			*req.AcceptanceRadiusMeters > maxAcceptanceRadiusM {
			return fmt.Errorf("%w: el radio de aceptación debe estar entre 5 y 500 metros", ErrCompetitionInvalid)
		}
		if req.Metric != nil {
			return fmt.Errorf("%w: type=route no admite metric", ErrCompetitionInvalid)
		}
	}
	if req.EndAt != nil && !req.EndAt.After(req.StartAt) {
		return fmt.Errorf("%w: end_at debe ser posterior a start_at", ErrCompetitionInvalid)
	}
	return nil
}

// resolvedCompetition empaca los batch-read de la competencia un detalle.
type competitionBundle struct {
	comp         *dbs.Competition
	participants []int64
	winners      []int64
}

func (s *competitionService) loadBundle(ctx *gin.Context, id int64) (*competitionBundle, error) {
	comp, err := s.dao.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	participants, winners, err := s.loadMembers(ctx, []int64{comp.ID})
	if err != nil {
		return nil, err
	}
	return &competitionBundle{comp: comp, participants: participants[comp.ID], winners: winners[comp.ID]}, nil
}

func (s *competitionService) loadMembers(ctx *gin.Context, ids []int64) (map[int64][]int64, map[int64][]int64, error) {
	pRows, err := s.dao.ListParticipantsByCompetitionIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	wRows, err := s.dao.ListWinnersByCompetitionIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	participants := make(map[int64][]int64, len(ids))
	for _, r := range pRows {
		participants[r.CompetitionID] = append(participants[r.CompetitionID], r.UserID)
	}
	winners := make(map[int64][]int64, len(ids))
	for _, r := range wRows {
		winners[r.CompetitionID] = append(winners[r.CompetitionID], r.UserID)
	}
	for i := range ids {
		if participants[ids[i]] == nil {
			participants[ids[i]] = []int64{}
		}
		if winners[ids[i]] == nil {
			winners[ids[i]] = []int64{}
		}
	}
	return participants, winners, nil
}

func (s *competitionService) Approve(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	bundle, guardErr := s.moderationGuard(ctx, callerID, competitionID)
	if guardErr != nil {
		return nil, guardErr
	}
	updated, err := s.dao.Approve(ctx, competitionID, callerID)
	if err != nil {
		customlogger.Error(ctx, "error approving competition", err, customlogger.TagMethod("CompetitionApprove"))
		return nil, fmt.Errorf("error al aprobar la competencia")
	}
	if !updated {
		return nil, fmt.Errorf("%w: solo se puede aprobar una competencia en pending_approval", ErrCompetitionConflict)
	}
	// Respuesta armada desde el bundle ya cargado: la consulta con re-guard
	// del detalle puede 403 cuando el caller es trainer creador no-owner (o
	// cualquier caso de guard distinto), y no hay razón para releer.
	bundle.comp.Status = competition.StatusActive
	bundle.comp.ApprovedByUserID = &callerID
	s.notifyCreator(bundle.comp, "Competencia aprobada", "Tu competencia fue aprobada y ya está activa.", "competition_approved")
	return s.detailResponse(ctx, bundle), nil
}

func (s *competitionService) Reject(ctx *gin.Context, callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error) {
	bundle, guardErr := s.moderationGuard(ctx, callerID, competitionID)
	if guardErr != nil {
		return nil, guardErr
	}
	updated, err := s.dao.Reject(ctx, competitionID, reason)
	if err != nil {
		customlogger.Error(ctx, "error rejecting competition", err, customlogger.TagMethod("CompetitionReject"))
		return nil, fmt.Errorf("error al rechazar la competencia")
	}
	if !updated {
		return nil, fmt.Errorf("%w: solo se puede rechazar una competencia en pending_approval", ErrCompetitionConflict)
	}
	bundle.comp.Status = competition.StatusRejected
	bundle.comp.RejectionReason = reason
	s.notifyCreator(bundle.comp, "Competencia rechazada", "Tu competencia propuesta fue rechazada.", "competition_rejected")
	return s.detailResponse(ctx, bundle), nil
}

// moderationGuard resuelve 404 y el guard de moderación (owner del team, o el
// trainer creador como caso defensivo) antes de tocar DAO de escritura.
func (s *competitionService) moderationGuard(ctx *gin.Context, callerID, competitionID int64) (*competitionBundle, error) {
	comp, err := s.dao.FindByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	isOwner, err := s.dao.IsTeamOwner(ctx, comp.TeamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al moderar la competencia")
	}
	isTrainerCreator := comp.CreatedByRole == competition.CreatorRoleTrainer && comp.CreatedByUserID == callerID
	if !isOwner && !isTrainerCreator {
		return nil, ErrCompetitionForbidden
	}
	participants, winners, err := s.loadMembers(ctx, []int64{comp.ID})
	if err != nil {
		return nil, fmt.Errorf("error al moderar la competencia")
	}
	return &competitionBundle{comp: comp, participants: participants[comp.ID], winners: winners[comp.ID]}, nil
}

func (s *competitionService) Cancel(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	comp, err := s.dao.FindByID(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	// Solo el creador corredor cancela; el owner NO puede (spec: "solo el
	// creador corredor, solo sobre pending").
	if !(comp.CreatedByUserID == callerID && comp.CreatedByRole == competition.CreatorRoleRunner) {
		return nil, ErrCompetitionForbidden
	}
	if comp.Status != competition.StatusPendingApproval {
		return nil, fmt.Errorf("%w: solo se puede cancelar una competencia en pending_approval", ErrCompetitionConflict)
	}
	if err := s.dao.Delete(ctx, competitionID); err != nil {
		customlogger.Error(ctx, "error deleting competition", err, customlogger.TagMethod("CompetitionCancel"))
		return nil, fmt.Errorf("error al cancelar la competencia")
	}
	return nil, nil
}

func (s *competitionService) ListByTeam(ctx *gin.Context, callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error) {
	var statusFilter *string
	if status != "" {
		validStatus := false
		for _, v := range competition.GetValidStatuses() {
			if v == status {
				validStatus = true
			}
		}
		if !validStatus {
			return nil, fmt.Errorf("%w: status debe ser una de: pending_approval, active, finished, rejected", ErrCompetitionInvalid)
		}
		statusFilter = &status
	}

	// Guard: miembro de algún grupo del team O owner (403 otro caso).
	isMember, err := s.dao.IsTeamGroupMember(ctx, teamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar las competencias")
	}
	if !isMember {
		isOwner, err := s.dao.IsTeamOwner(ctx, teamID, callerID)
		if err != nil {
			return nil, fmt.Errorf("error al consultar las competencias")
		}
		if !isOwner {
			return nil, ErrCompetitionForbidden
		}
	}

	hasUnseen, err := s.dao.HasUnseenTeam(ctx, callerID, teamID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar las competencias")
	}
	comps, err := s.dao.ListByTeam(ctx, teamID, statusFilter, groupID)
	if err != nil {
		customlogger.Error(ctx, "error listing competitions", err, customlogger.TagMethod("CompetitionListByTeam"))
		return nil, fmt.Errorf("error al consultar las competencias")
	}
	ids := make([]int64, 0, len(comps))
	for _, c := range comps {
		ids = append(ids, c.ID)
	}
	participants, winners, err := s.loadMembers(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("error al consultar las competencias")
	}
	items := make([]competition.CompetitionResponse, 0, len(comps))
	for i := range comps {
		items = append(items, *toCompetitionResponse(&comps[i], participants[comps[i].ID], winners[comps[i].ID]))
	}
	return &competition.CompetitionsListResponse{Data: items, HasUnseen: hasUnseen}, nil
}

func (s *competitionService) Get(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionDetailResponse, error) {
	bundle, err := s.loadBundle(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	// Guard: miembro del team u owner (403) — 404 si no existe ya subió.
	isOwner, err := s.dao.IsTeamOwner(ctx, bundle.comp.TeamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar la competencia")
	}
	if !isOwner {
		isMember, err := s.dao.IsTeamGroupMember(ctx, bundle.comp.TeamID, callerID)
		if err != nil {
			return nil, fmt.Errorf("error al consultar la competencia")
		}
		if !isMember {
			return nil, ErrCompetitionForbidden
		}
	}
	return &competition.CompetitionDetailResponse{Data: *s.detailResponse(ctx, bundle)}, nil
}

// detailResponse arma la respuesta del detalle desde el bundle (usada en Get
// y por las mutaciones que ya cargaron el bundle: sin re-Get ni re-guard).
func (s *competitionService) detailResponse(ctx *gin.Context, bundle *competitionBundle) *competition.CompetitionResponse {
	resp := toCompetitionResponse(bundle.comp, bundle.participants, bundle.winners)
	if bundle.comp.Type == competition.TypeMetric {
		resp.Progress = s.computeMetricProgress(ctx, bundle)
	} else if bundle.comp.Type == competition.TypeRoute {
		resp.Progress = s.computeRouteProgress(ctx, bundle)
	}
	return resp
}

// computeMetricProgress arma el acumulado in-ventana por participante
// (detalle on-demand; sin broadcast). El progreso de route corre su matcher.
func (s *competitionService) computeMetricProgress(ctx *gin.Context, bundle *competitionBundle) []competition.CompetitionProgressItem {
	agg, err := s.dao.AggregateFeedbackWindow(ctx, bundle.comp.TeamID, bundle.participants, gametime.DateOnly(bundle.comp.StartAt), gametime.DateOnly(windowEnd(bundle.comp)))
	if err != nil {
		customlogger.Error(ctx, "error computing competition progress", err, customlogger.TagMethod("CompetitionGet"))
		return nil
	}
	byUser := make(map[int64]dbs.CompetitionMetricAgg, len(agg))
	for _, r := range agg {
		byUser[r.UserID] = r
	}
	progress := make([]competition.CompetitionProgressItem, 0, len(bundle.participants))
	for _, uid := range bundle.participants {
		value := metricValueFromAgg(byUser[uid], bundle.comp.Metric)
		progress = append(progress, competition.CompetitionProgressItem{UserID: uid, Value: value})
	}
	sort.Slice(progress, func(i, j int) bool { return progress[i].UserID < progress[j].UserID })
	return progress
}

// competitionRouteAttempt es un intento de competencia tipo route: el valor
// matcheado y la duración del intento (desempate D10).
type competitionRouteAttempt struct {
	userID     int64
	pct        float64
	durationMs float64
}

// routeAttempts arma los intentos de la competencia a partir de los runner
// sessions tagueados: 1 batch de sesiones + 1 batch de puntos, sin N+1.
// Devuelve también los instance_ids de TODOS los intentos registrados (canales
// WS del cierre). Solo los intentos finished de participantes computan match.
func (s *competitionService) routeAttempts(ctx *gin.Context, bundle *competitionBundle) (attempts []competitionRouteAttempt, registeredInstanceIDs []int64, err error) {
	sessions, err := s.dao.ListAttemptSessions(ctx, bundle.comp.ID)
	if err != nil {
		return nil, nil, err
	}
	seenInstances := make(map[int64]bool, len(sessions))
	inParticipants := make(map[int64]bool, len(bundle.participants))
	for _, uid := range bundle.participants {
		inParticipants[uid] = true
	}
	finished := make([]dbs.RunnerSession, 0, len(sessions))
	for _, rs := range sessions {
		if !seenInstances[rs.SessionInstanceID] {
			seenInstances[rs.SessionInstanceID] = true
			registeredInstanceIDs = append(registeredInstanceIDs, rs.SessionInstanceID)
		}
		if !inParticipants[rs.AthleteUserID] || rs.Status != runnersession.RunnerSessionStatusFinished {
			continue
		}
		finished = append(finished, rs)
	}
	if len(finished) == 0 {
		return nil, registeredInstanceIDs, nil
	}
	instanceIDs := make([]int64, 0, len(finished))
	for _, rs := range finished {
		instanceIDs = append(instanceIDs, rs.SessionInstanceID)
	}
	points, err := s.dao.ListAttemptPoints(ctx, instanceIDs, bundle.participants)
	if err != nil {
		return nil, nil, err
	}
	byAttempt := make(map[[2]int64][]routematch.Point, len(points))
	for _, p := range points {
		key := [2]int64{p.SessionInstanceID, p.AthleteUserID}
		byAttempt[key] = append(byAttempt[key], routematch.Point{Lat: p.Latitude, Lng: p.Longitude})
	}
	samples, err := s.routeSamples(ctx, bundle.comp)
	if err != nil {
		return nil, nil, err
	}
	radius := 0.0
	if bundle.comp.AcceptanceRadiusMeters != nil {
		radius = *bundle.comp.AcceptanceRadiusMeters
	}
	attempts = make([]competitionRouteAttempt, 0, len(finished))
	for _, rs := range finished {
		pct := routematch.MatchPercent(samples, radius, byAttempt[[2]int64{rs.SessionInstanceID, rs.AthleteUserID}])
		duration := 0.0
		if rs.EndDate != nil && rs.EndDate.After(rs.StartDate) {
			duration = rs.EndDate.Sub(rs.StartDate).Seconds() * 1000
		}
		attempts = append(attempts, competitionRouteAttempt{userID: rs.AthleteUserID, pct: pct, durationMs: duration})
	}
	return attempts, registeredInstanceIDs, nil
}

// routeSamples parsea y samplea el GeoJSON de la competencia; el fallo de
// parseo (ruta mal cargada en el alta) loguea y se propaga: cerrar/detallar
// con un matching sin ruta confiable sería mentir el %.
func (s *competitionService) routeSamples(ctx *gin.Context, comp *dbs.Competition) ([]routematch.Point, error) {
	if comp.RouteGeoJSON == nil {
		return nil, fmt.Errorf("competencia %d sin route_geojson", comp.ID)
	}
	route, err := routematch.ParseLineString(*comp.RouteGeoJSON)
	if err != nil {
		customlogger.Error(ctx, "error parsing competition route", err, customlogger.TagMethod("CompetitionRouteMatch"))
		return nil, fmt.Errorf("route_geojson inválido: %w", err)
	}
	return routematch.SampleRoute(route), nil
}

// computeRouteProgress arma el mejor % de match de cada participante hasta
// ahora (detalle on-demand; el matching es el mismo del cierre, D10).
func (s *competitionService) computeRouteProgress(ctx *gin.Context, bundle *competitionBundle) []competition.CompetitionProgressItem {
	attempts, _, err := s.routeAttempts(ctx, bundle)
	if err != nil {
		customlogger.Error(ctx, "error computing competition route progress", err, customlogger.TagMethod("CompetitionGet"))
		return nil
	}
	best := make(map[int64]float64, len(attempts))
	for _, a := range attempts {
		if a.pct > best[a.userID] {
			best[a.userID] = a.pct
		}
	}
	progress := make([]competition.CompetitionProgressItem, 0, len(bundle.participants))
	for _, uid := range bundle.participants {
		value := best[uid]
		progress = append(progress, competition.CompetitionProgressItem{UserID: uid, Value: &value})
	}
	sort.Slice(progress, func(i, j int) bool { return progress[i].UserID < progress[j].UserID })
	return progress
}

// windowEnd devuelve end_at o now (ventana "[start_at, end_at o now]").
func windowEnd(comp *dbs.Competition) time.Time {
	if comp.EndAt != nil && comp.EndAt.Before(gametime.Now()) {
		return *comp.EndAt
	}
	return gametime.Now()
}

func metricValueFromAgg(agg dbs.CompetitionMetricAgg, metric *string) *float64 {
	if metric == nil {
		return nil
	}
	switch *metric {
	case "distance":
		return agg.SumDistance
	case "duration":
		if agg.SumDurationMs == nil {
			return nil
		}
		v := float64(*agg.SumDurationMs)
		return &v
	case "pace":
		if agg.SumDurationMs == nil || agg.SumDistance == nil {
			return nil
		}
		d := *agg.SumDistance
		if d <= 0 {
			return nil
		}
		v := float64(*agg.SumDurationMs) / d
		return &v
	case "peak_speed":
		return agg.MaxPeak
	}
	return nil
}

func (s *competitionService) Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, []int64, error) {
	bundle, err := s.loadBundle(ctx, competitionID)
	if err != nil {
		return nil, nil, err
	}
	// Guard: creador (trainer o runner) u owner del team.
	isOwner, err := s.dao.IsTeamOwner(ctx, bundle.comp.TeamID, callerID)
	if err != nil {
		return nil, nil, fmt.Errorf("error al cerrar la competencia")
	}
	if !isOwner && bundle.comp.CreatedByUserID != callerID {
		return nil, nil, ErrCompetitionForbidden
	}
	if bundle.comp.Status != competition.StatusActive {
		return nil, nil, fmt.Errorf("%w: solo se puede cerrar una competencia active", ErrCompetitionConflict)
	}
	// Cierre manual pedido antes de que el end_at pase (D9: manual; si el
	// front lo intenta antes de que la ventana termine, 400).
	if bundle.comp.EndAt != nil && gametime.Now().Before(*bundle.comp.EndAt) {
		return nil, nil, fmt.Errorf("%w: competencia no puede cerrarse todavía", ErrCompetitionInvalid)
	}

	// Intentos registrados: canales WS del cierre (solo si hay).
	attemptSessions, err := s.dao.ListAttemptSessions(ctx, competitionID)
	if err != nil {
		customlogger.Error(ctx, "error listing competition attempts", err, customlogger.TagMethod("CompetitionClose"))
		return nil, nil, fmt.Errorf("error al cerrar la competencia")
	}
	registeredInstanceIDs := make([]int64, 0, len(attemptSessions))
	seen := make(map[int64]bool, len(attemptSessions))
	for _, rs := range attemptSessions {
		if !seen[rs.SessionInstanceID] {
			seen[rs.SessionInstanceID] = true
			registeredInstanceIDs = append(registeredInstanceIDs, rs.SessionInstanceID)
		}
	}

	// Winners: metric por ventana / route por matching. Fallo aquí (ex. ruta
	// mal cargada) sube: cerrar sin matching confiable mentiría el resultado.
	winners, err := s.computeWinners(ctx, bundle)
	if err != nil {
		return nil, nil, fmt.Errorf("error al cerrar la competencia")
	}
	finishedAt := gametime.Now()
	// Efectos en la MISMA transacción del finish: winners, stamps (solo con
	// ganador) y unseen para todos los participantes (D9).
	stamps, unseen := closeEffects(bundle, winners, finishedAt)
	updated, err := s.dao.Finish(ctx, competitionID, finishedAt, winners, stamps, unseen)
	if err != nil {
		customlogger.Error(ctx, "error closing competition", err, customlogger.TagMethod("CompetitionClose"))
		return nil, nil, fmt.Errorf("error al cerrar la competencia")
	}
	if !updated {
		return nil, nil, fmt.Errorf("%w: solo se puede cerrar una competencia active", ErrCompetitionConflict)
	}

	if len(winners) > 0 {
		s.notifyFinish(bundle.participants, competitionID)
	}

	// Respuesta directo del bundle (sin re-Get: el guard de detalle puede
	// 403 a trainer creador no-owner — mismo fix de la moderación).
	bundle.comp.Status = competition.StatusFinished
	bundle.comp.FinishedAt = &finishedAt
	bundle.winners = winners
	return s.detailResponse(ctx, bundle), registeredInstanceIDs, nil
}

// computeWinners compara la métrica (o el match % de route) entre participantes.
// Empate EXACTO → todos ganan (D9). Value null (sin datos) = sin candidato.
// type=metric usa la misma agregación en ventana que el progreso del detalle
// (criterios del leaderboard D6); pace menor = mejor.
func (s *competitionService) computeWinners(ctx *gin.Context, bundle *competitionBundle) ([]int64, error) {
	switch bundle.comp.Type {
	case competition.TypeMetric:
		agg, err := s.dao.AggregateFeedbackWindow(ctx, bundle.comp.TeamID, bundle.participants, gametime.DateOnly(bundle.comp.StartAt), gametime.DateOnly(windowEnd(bundle.comp)))
		if err != nil {
			return nil, err
		}
		byUser := make(map[int64]*float64, len(agg))
		for _, r := range agg {
			byUser[r.UserID] = metricValueFromAgg(r, bundle.comp.Metric)
		}
		best := bestMetricCandidates(bundle.participants, bundle.comp.Metric, func(uid int64) *float64 {
			return byUser[uid]
		})
		return best, nil
	case competition.TypeRoute:
		attempts, _, err := s.routeAttempts(ctx, bundle)
		if err != nil {
			return nil, err
		}
		if len(attempts) == 0 {
			return nil, nil
		}
		bestPct := attempts[0].pct
		for _, a := range attempts[1:] {
			if a.pct > bestPct {
				bestPct = a.pct
			}
		}
		// Tie-break de duración SOLO dentro del subconjunto del mejor %.
		bestDur := 0.0
		for _, a := range attempts {
			if a.pct == bestPct && (bestDur == 0 || a.durationMs < bestDur) {
				bestDur = a.durationMs
			}
		}
		winners := make([]int64, 0, len(attempts))
		for _, a := range attempts {
			if a.pct == bestPct && a.durationMs == bestDur {
				winners = append(winners, a.userID)
			}
		}
		return winners, nil
	}
	return nil, nil
}

// bestMetricCandidates devuelve todos los usuarios empatados en el mejor valor
// de métrica (igualdad EXACTA: D9 empate → todos ganan). Menor pace = mejor.
func bestMetricCandidates(participants []int64, metric *string, valueOf func(int64) *float64) []int64 {
	candidates := make([]int64, 0, len(participants))
	var best *float64
	for _, uid := range participants {
		v := valueOf(uid)
		if v == nil {
			continue
		}
		if best == nil {
			best = v
			candidates = append(candidates, uid)
			continue
		}
		if isBetterMetricValue(metric, *v, *best) {
			best = v
			candidates = candidates[:0] // reset slice length
			candidates = append(candidates, uid)
		} else if *v == *best {
			candidates = append(candidates, uid)
		}
	}
	return candidates
}

// isBetterMetricValue indica si el valor a supera a b dentro del ranking del
// metric dado. Los 4 son fuente de feedback (D6); pace menor mejor.
func isBetterMetricValue(metric *string, a, b float64) bool {
	if metric != nil && *metric == "pace" {
		return a < b
	}
	return a > b
}

// closeEffects arma stamps (solo si hay ganadores) y unseen (todos los
// participantes) — filas para la tx de Finish del DAO.
func closeEffects(bundle *competitionBundle, winners []int64, finishedAt time.Time) ([]dbs.GamificationStamp, []dbs.GamificationUnseen) {
	var stamps []dbs.GamificationStamp
	if len(winners) > 0 {
		stamps = make([]dbs.GamificationStamp, 0, len(winners))
		for _, uid := range winners {
			stamps = append(stamps, dbs.GamificationStamp{
				UserID:     uid,
				Type:       "competition",
				SourceID:   bundle.comp.ID,
				SourceName: fmt.Sprintf("Competencia %d", bundle.comp.ID),
				GrantedAt:  finishedAt,
				Count:      1,
			})
		}
	}
	unseen := make([]dbs.GamificationUnseen, 0, len(bundle.participants))
	for _, uid := range bundle.participants {
		unseen = append(unseen, dbs.GamificationUnseen{
			UserID:  uid,
			Section: constants.GamificationSectionCompetitions,
			TeamID:  &bundle.comp.TeamID,
		})
	}
	return stamps, unseen
}

// notifyFinish es el push best-effort del cierre a los participantes: texto
// genérico (D8: sin title, "Competencia Finalizada"), data.route placeholder y
// fail silencioso con log.
func (s *competitionService) notifyFinish(participants []int64, competitionID int64) {
	if s.pushTokenDao == nil || s.pushClient == nil {
		return
	}
	route := fmt.Sprintf("/competencias/%d", competitionID)
	for _, uid := range participants {
		sendPushToUser(nil, s.pushTokenDao, s.pushClient, uid,
			"Competencia finalizada", "La competencia cerró y ya podés ver el resultado.", "competition_finished", route)
	}
}

// notifyCreator es el push best-effort de moderación hacia el corredor
// creador: nil-safe (sin tokens/push configurados no hace nada) y el fallo
// se loguea sin bloquear la respuesta.
func (s *competitionService) notifyCreator(comp *dbs.Competition, title, body, notifType string) {
	if s.pushTokenDao == nil || s.pushClient == nil {
		return
	}
	sendPushToUser(nil, s.pushTokenDao, s.pushClient, comp.CreatedByUserID,
		title, body, notifType, teamRoute(comp.TeamID))
}

func teamRoute(teamID int64) string {
	return fmt.Sprintf("/teams/%d", teamID)
}

func toCompetitionResponse(comp *dbs.Competition, participants, winners []int64) *competition.CompetitionResponse {
	return &competition.CompetitionResponse{
		ID:                     comp.ID,
		TeamID:                 comp.TeamID,
		GroupID:                comp.GroupID,
		Type:                   comp.Type,
		Metric:                 comp.Metric,
		StartAt:                comp.StartAt,
		EndAt:                  comp.EndAt,
		Route:                  comp.RouteGeoJSON,
		AcceptanceRadiusMeters: comp.AcceptanceRadiusMeters,
		Participants:           participants,
		CreatedByUserID:        comp.CreatedByUserID,
		CreatedByRole:          comp.CreatedByRole,
		Status:                 comp.Status,
		ApprovedByUserID:       comp.ApprovedByUserID,
		RejectionReason:        comp.RejectionReason,
		WinnerUserIDs:          winners,
		FinishedAt:             comp.FinishedAt,
	}
}
