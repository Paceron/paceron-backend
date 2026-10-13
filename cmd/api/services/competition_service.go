package services

// Competencias (gamificacion-core E4.1, D8/D9). CRUD + moderación + cierre
// stub (la computación de winners llega en E4.2).

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
	"simple-arq-golang/cmd/api/utils/gametime"
)

var (
	ErrCompetitionInvalid   = errors.New("datos de competencia inválidos")
	ErrCompetitionForbidden = errors.New("no tenés permisos para operar sobre esta competencia")
	ErrCompetitionConflict  = errors.New("transición de estado inválida")
)

// métricas válidas para type=metric: las mismas fuente de feedback (D6/D7).
var validCompetitionMetrics = []string{"distance", "duration", "pace", "peak_speed"}

type CompetitionServiceInterface interface {
	Create(ctx *gin.Context, callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error)
	Approve(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error)
	Reject(ctx *gin.Context, callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error)
	Cancel(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error)
	ListByTeam(ctx *gin.Context, callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error)
	Get(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionDetailResponse, error)
	Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error)
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
// metric + end_at > start_at; route exige route_geojson + radius > 0.
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
		if req.AcceptanceRadiusMeters == nil || *req.AcceptanceRadiusMeters <= 0 {
			return fmt.Errorf("%w: type=route exige acceptance_radius_meters > 0", ErrCompetitionInvalid)
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
	s.notifyCreator(bundle.comp, "Competencia aprobada", "Tu competencia fue aprobada y ya está activa.", "competition_approved")
	detail, err := s.Get(ctx, callerID, competitionID)
	if err != nil {
		return nil, err
	}
	return &detail.Data, nil
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
	s.notifyCreator(bundle.comp, "Competencia rechazada", "Tu competencia propuesta fue rechazada.", "competition_rejected")
	detail, err := s.Get(ctx, callerID, competitionID)
	if err != nil {
		return nil, err
	}
	return &detail.Data, nil
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
	return &competitionBundle{comp: comp}, nil
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
	resp := toCompetitionResponse(bundle.comp, bundle.participants, bundle.winners)
	if bundle.comp.Type == competition.TypeMetric {
		resp.Progress = s.computeMetricProgress(ctx, bundle)
	}
	return &competition.CompetitionDetailResponse{Data: *resp}, nil
}

// computeMetricProgress arma el acumulado in-ventana por participante
// (detalle on-demand; sin broadcast). route queda sin progreso hasta E4.2.
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

func (s *competitionService) Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	bundle, err := s.loadBundle(ctx, competitionID)
	if err != nil {
		return nil, err
	}
	// Guard: creador (trainer o runner) u owner del team.
	isOwner, err := s.dao.IsTeamOwner(ctx, bundle.comp.TeamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al cerrar la competencia")
	}
	if !isOwner && bundle.comp.CreatedByUserID != callerID {
		return nil, ErrCompetitionForbidden
	}
	if bundle.comp.Status != competition.StatusActive {
		return nil, fmt.Errorf("%w: solo se puede cerrar una competencia active", ErrCompetitionConflict)
	}
	// Cierre manual pedido antes de que el end_at pase (D9: manual; si el
	// front lo intenta antes de que la ventana termine, 400).
	if bundle.comp.EndAt != nil && gametime.Now().Before(*bundle.comp.EndAt) {
		return nil, fmt.Errorf("%w: competencia no puede cerrarse todavía", ErrCompetitionInvalid)
	}
	updated, err := s.dao.Finish(ctx, competitionID, gametime.Now())
	if err != nil {
		customlogger.Error(ctx, "error closing competition", err, customlogger.TagMethod("CompetitionClose"))
		return nil, fmt.Errorf("error al cerrar la competencia")
	}
	if !updated {
		return nil, fmt.Errorf("%w: solo se puede cerrar una competencia active", ErrCompetitionConflict)
	}
	detail, err := s.Get(ctx, callerID, competitionID)
	if err != nil {
		return nil, err
	}
	return &detail.Data, nil
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
