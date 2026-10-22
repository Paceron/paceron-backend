package daos

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// ErrCompetitionNotFound se devuelve cuando la competencia no existe. El
// controller lo mapea a 404.
var ErrCompetitionNotFound = errors.New("competencia no encontrada")

// ErrCompetitionAttemptLimit alcanza el atleta su max_attempts (Gap 36). El
// service lo envuelve en ErrCompetitionInvalid (→ 400 con este mismo mensaje);
// no se mapea directo en controller para dejar el mapping de intentos intacto.
var ErrCompetitionAttemptLimit = errors.New("llegaste al límite de intentos de esta competencia")

// ErrCompetitionAttemptConflict es el ON CONFLICT del intento 1 (instancia
// compartida) que no insertó: el atleta ya tiene runner_session sobre esa
// instancia SIN tag de competencia. El service lo envuelve en
// ErrCompetitionInvalid → 400 "ya tenés un intento registrado...".
var ErrCompetitionAttemptConflict = errors.New("ya tenés un intento registrado en esta competencia")

func isGormNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// CompetitionDAOInterface es el acceso a datos de competencias (gamificacion-core
// D8/D9). La integridad de participants la gobierna el service: Create inserta
// competencia + participantes en una transacción.
type CompetitionDAOInterface interface {
	// Chequeos de guard (propios para evitar dependencias entre DAOs).
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error)
	// CountTeamGroupMembers cuenta cuántos de los userIDs pasados son miembros
	// activos de algún grupo del team (criterio del leaderboard guard).
	CountTeamGroupMembers(ctx *gin.Context, teamID int64, userIDs []int64) (int64, error)
	// GroupBelongsToTeam valida que el group_id del body apunte a un grupo
	// del mismo team.
	GroupBelongsToTeam(ctx *gin.Context, groupID, teamID int64) (bool, error)

	// Create inserta la competencia y sus participantes en una transacción.
	Create(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error
	// CreateWithInstance es el Create extendido de competencias-ruta-intake
	// D1: dentro de la MISMA transacción, para type=route crea la instancia
	// sintética (session_instance + exercise_instance "Corrida" + link
	// role=main) y persiste su id en route_session_instance_id; para
	// type=metric no crea instancia ni toca la columna. La creación fallida
	// de cualquiera de las piezas revierte todo (competencia + participantes
	// + instancia: nada queda huérfano).
	CreateWithInstance(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Competition, error)
	// FindByRouteSessionInstance resuelve la competencia type=route dueña de
	// la instancia sintética dada (competencias-ruta-intake, guard
	// bidireccional del intento). ErrCompetitionNotFound si ninguna route
	// ancla esa instancia (instancia regular de calendario).
	FindByRouteSessionInstance(ctx *gin.Context, sessionInstanceID int64) (*dbs.Competition, error)
	// ListParticipantsByCompetitionIDs trae las membresías en batch (sin N+1
	// en detalle ni listado).
	ListParticipantsByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionParticipant, error)
	ListWinnersByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionWinner, error)
	// ListByTeam aplica los filtros opcionales status y group_id.
	ListByTeam(ctx *gin.Context, teamID int64, status *string, groupID *int64) ([]dbs.Competition, error)
	// IsParticipant indica si el usuario está entre los participantes.
	IsParticipant(ctx *gin.Context, competitionID, userID int64) (bool, error)
	// ExistsAttemptByCompetitionAndAthlete consulta si el atleta ya tiene
	// runner_session tagueada con esta competencia (cualquier status).
	ExistsAttemptByCompetitionAndAthlete(ctx *gin.Context, competitionID, athleteUserID int64) (bool, error)
	// CreateAttemptSession materializa un intento del atleta (Gap 36) en una
	// sola transacción: lock FOR UPDATE de la fila de competencia (serializa
	// concurrentes — el unique (session_instance_id, athlete_user_id) ya no
	// arbitra el límite porque los intentos 2+ usan instancias distintas),
	// COUNT de intentos (cualquier status) y decisión del límite con el
	// max_attempts FRESCO de la fila lockeada (ErrCompetitionAttemptLimit si
	// count >= max_attempts). Intento 1 reusa la instancia compartida (ON
	// CONFLICT DO NOTHING, same-as-intake); intento 2+ crea instancia nueva
	// "Competencia ruta N (intento M)". Cualquier fallo → rollback total.
	CreateAttemptSession(ctx *gin.Context, comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error)
	// Approve / Reject / Finish aplican la transición de status (guard de
	// status en el WHERE: solo corren sobre el status origen válido) y
	// devuelven si hubo cambio. Finish es el cierre E4.2 race-safe: una única
	// transacción persiste la transición + winners + stamps + unseen
	// (all-or-nothing). Los efectos nulos son no-op.
	Approve(ctx *gin.Context, competitionID, approvedByUserID int64) (bool, error)
	Reject(ctx *gin.Context, competitionID int64, reason *string) (bool, error)
	Finish(ctx *gin.Context, competitionID int64, finishedAt time.Time, winners []int64, stamps []dbs.GamificationStamp, unseen []dbs.GamificationUnseen) (bool, error)

	// ListAttemptSessions devuelve los runner sessions tagueados con
	// competition_id (intentos registrados de cualquier status; el service
	// filtra finished para el matching — D9/D10).
	ListAttemptSessions(ctx *gin.Context, competitionID int64) ([]dbs.RunnerSession, error)
	// ListAttemptsByCompetitionAndAthlete devuelve los intentos del atleta en
	// la competencia (cualquier status), ORDER BY id ASC (cronológico =
	// número de intento, Gap 36 D4).
	ListAttemptsByCompetitionAndAthlete(ctx *gin.Context, competitionID, athleteUserID int64) ([]dbs.RunnerSession, error)
	// SumFeedbackDistanceByInstances suma distance_meters del feedback activo
	// del atleta sobre las instancias dadas, GROUP BY assigned_session_id
	// (batch sin N+1, D4). Sin feedback para una instancia no hay fila.
	SumFeedbackDistanceByInstances(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.CompetitionInstanceDistanceAgg, error)
	// ListAttemptPoints trae en una sola query los puntos GPS de los intentos
	// provistos (join feedback activo del atleta, D10).
	ListAttemptPoints(ctx *gin.Context, sessionInstanceIDs, athleteUserIDs []int64) ([]dbs.CompetitionAttemptPoint, error)
	// CountAttemptsByCompetitionAndAthlete cuenta los intentos (cualquier
	// status) del atleta en UNA sola query para todas las competencias pedidas
	// (Gap 36 D7): LEFT JOIN competition_participant garantiza fila para todo
	// participante (count 0 sin runner_session) y ninguna fila para el
	// no-participante — el service mapea fila→0..N y ausencia→null.
	CountAttemptsByCompetitionAndAthlete(ctx *gin.Context, competitionIDs []int64, athleteUserID int64) (map[int64]int64, error)
	// Delete borra físicamente la competencia junto con sus participantes
	// (DELETE cancel D9; borrado físico). El guard de status corre en el SQL
	// (solo sobre pending_approval) para arbitrar la carrera cancel-vs-approve:
	// 0 filas = ya no estaba pending y no se borra nada (devuelve false). En la
	// MISMA transacción limpia la instancia sintética route que quedó apuntando
	// la competencia (sin intentos por ser solo-pending: no deja huérfanos).
	Delete(ctx *gin.Context, competitionID int64) (bool, error)

	// TeamOwnerID devuelve el owner del team (0 si el team no existe): el
	// service lo usa para excluirlo de los participants —el entrenador no
	// compite— sin importar quién sea el creador.
	TeamOwnerID(ctx *gin.Context, teamID int64) (int64, error)

	// HasUnseenTeam es el EXISTS de novedades (D11) para el listado embebido.
	HasUnseenTeam(ctx *gin.Context, userID, teamID int64) (bool, error)

	// AggregateFeedbackWindow acumula métricas de feedback por atleta dentro
	// de la ventana del team (detalle on-demand D9).
	AggregateFeedbackWindow(ctx *gin.Context, teamID int64, participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error)
}

type competitionDao struct {
	DB *gorm.DB
}

// NewCompetitionDao crea una nueva instancia de CompetitionDao.
func NewCompetitionDao(database *gorm.DB) CompetitionDAOInterface {
	return &competitionDao{DB: database}
}

func (d *competitionDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Team{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition team owner: %w", err)
	}
	return count > 0, nil
}

func (d *competitionDao) TeamOwnerID(ctx *gin.Context, teamID int64) (int64, error) {
	var team dbs.Team
	err := d.DB.Select("owner_id").Where("id = ? AND deleted_at IS NULL", teamID).First(&team).Error
	if err != nil {
		if isGormNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("error fetching competition team owner: %w", err)
	}
	return team.OwnerID, nil
}

func (d *competitionDao) IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.GroupUser{}).
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("groups.team_id = ? AND group_users.user_id = ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition group membership: %w", err)
	}
	return count > 0, nil
}

func (d *competitionDao) CountTeamGroupMembers(ctx *gin.Context, teamID int64, userIDs []int64) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	var count int64
	err := d.DB.Model(&dbs.GroupUser{}).
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("groups.team_id = ? AND group_users.user_id IN ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", teamID, userIDs).
		Distinct("group_users.user_id").
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting competition team members: %w", err)
	}
	return count, nil
}

func (d *competitionDao) GroupBelongsToTeam(ctx *gin.Context, groupID, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Group{}).
		Where("id = ? AND team_id = ? AND deleted_at IS NULL", groupID, teamID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition group belongs to team: %w", err)
	}
	return count > 0, nil
}

// Create inserta la competencia y sus participantes en una transacción. El
// fallo de cualquiera de las dos escrituras revierte ambas.
func (d *competitionDao) Create(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error {
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comp).Error; err != nil {
			return fmt.Errorf("error creating competition: %w", err)
		}
		participants := make([]dbs.CompetitionParticipant, 0, len(participantIDs))
		for _, id := range participantIDs {
			participants = append(participants, dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: id})
		}
		if err := tx.Create(&participants).Error; err != nil {
			return fmt.Errorf("error creating competition participants: %w", err)
		}
		return nil
	})
	return err
}

// CreateWithInstance persiste competencia + participantes y, para type=route
// (competencias-ruta-intake D1), la instancia sintética del intento — todo en
// la MISMA transacción: si la instancia falla, no queda competencia ni
// participantes huérfanos. El id de la instancia se persiste en
// route_session_instance_id con un UPDATE dentro del tx (la fila de
// competencia se inserta primero para obtener el PK que nombra la instancia).
func (d *competitionDao) CreateWithInstance(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error {
	isRoute := comp.Type == competition.TypeRoute
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comp).Error; err != nil {
			return fmt.Errorf("error creating competition: %w", err)
		}
		if isRoute {
			instance, err := createSyntheticRouteInstance(tx, fmt.Sprintf("Competencia ruta %d", comp.ID))
			if err != nil {
				return err
			}
			res := tx.Model(&dbs.Competition{}).
				Where("id = ?", comp.ID).
				Update("route_session_instance_id", instance.ID)
			if res.Error != nil {
				return fmt.Errorf("error saving competition route instance: %w", res.Error)
			}
			comp.RouteSessionInstanceID = &instance.ID
		}
		participants := make([]dbs.CompetitionParticipant, 0, len(participantIDs))
		for _, id := range participantIDs {
			participants = append(participants, dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: id})
		}
		if err := tx.Create(&participants).Error; err != nil {
			return fmt.Errorf("error creating competition participants: %w", err)
		}
		return nil
	})
	return err
}

// createSyntheticRouteInstance arma la estructura mínima compartida por todos
// los participantes de una competencia route (D1): 1 session_instance con el
// nombre dado + 1 exercise_instance sintético (name "Corrida", kind
// velocidad_distancia, intencionalmente sin datos de catálogo:
// SourceExerciseID NULL) + 1 link role=main. La instancia no pertenece a
// ningún día de calendario.
func createSyntheticRouteInstance(tx *gorm.DB, name string) (*dbs.SessionInstance, error) {
	sessionInstance := &dbs.SessionInstance{
		Name:        name,
		Description: strPtrDaos("Instancia sintética de la competencia de ruta"),
	}
	if err := tx.Create(sessionInstance).Error; err != nil {
		return nil, fmt.Errorf("error creating competition session instance: %w", err)
	}
	exerciseInstance := &dbs.ExerciseInstance{
		Name: "Corrida",
		Kind: string(constants.ExerciseKindVelocidadDistancia),
	}
	if err := tx.Create(exerciseInstance).Error; err != nil {
		return nil, fmt.Errorf("error creating competition exercise instance: %w", err)
	}
	link := &dbs.SessionExerciseInstance{
		SessionInstanceID:  sessionInstance.ID,
		ExerciseInstanceID: exerciseInstance.ID,
		Role:               string(constants.SessionExerciseRoleMain),
		RepeatCount:        1,
		RestMinutes:        0,
	}
	if err := tx.Create(link).Error; err != nil {
		return nil, fmt.Errorf("error creating competition exercise link: %w", err)
	}
	return sessionInstance, nil
}

// strPtrDaos helper de puntero de string de producción (instancia sintética).
func strPtrDaos(s string) *string { return &s }

func (d *competitionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Competition, error) {
	var comp dbs.Competition
	err := d.DB.Where("id = ?", id).First(&comp).Error
	if err != nil {
		if isGormNotFound(err) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("error finding competition: %w", err)
	}
	return &comp, nil
}

func (d *competitionDao) FindByRouteSessionInstance(ctx *gin.Context, sessionInstanceID int64) (*dbs.Competition, error) {
	var comp dbs.Competition
	err := d.DB.Where("route_session_instance_id = ? AND type = ?", sessionInstanceID, competition.TypeRoute).First(&comp).Error
	if err != nil {
		if isGormNotFound(err) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("error finding route competition by instance: %w", err)
	}
	return &comp, nil
}

func (d *competitionDao) ListParticipantsByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionParticipant, error) {
	if len(competitionIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionParticipant
	err := d.DB.Where("competition_id IN ?", competitionIDs).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition participants: %w", err)
	}
	return rows, nil
}

func (d *competitionDao) ListWinnersByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionWinner, error) {
	if len(competitionIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionWinner
	err := d.DB.Where("competition_id IN ?", competitionIDs).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition winners: %w", err)
	}
	return rows, nil
}

func (d *competitionDao) ListByTeam(ctx *gin.Context, teamID int64, status *string, groupID *int64) ([]dbs.Competition, error) {
	q := d.DB.Where("team_id = ?", teamID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	var comps []dbs.Competition
	err := q.Order("created_at DESC").Find(&comps).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competitions: %w", err)
	}
	return comps, nil
}

func (d *competitionDao) IsParticipant(ctx *gin.Context, competitionID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.CompetitionParticipant{}).
		Where("competition_id = ? AND user_id = ?", competitionID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition participant: %w", err)
	}
	return count > 0, nil
}

// ExistsAttemptByCompetitionAndAthlete consulta runner_session por la marca
// de competencia (FK opaca competition_id): es una tabla ajena al package
// pero la regla del intento es de la competencia (D2).
func (d *competitionDao) ExistsAttemptByCompetitionAndAthlete(ctx *gin.Context, competitionID, athleteUserID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.RunnerSession{}).
		Where("competition_id = ? AND athlete_user_id = ?", competitionID, athleteUserID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition attempt: %w", err)
	}
	return count > 0, nil
}

// CreateAttemptSession implementa la mecánica D2 del change
// competencias-ruta-intentos-multiples (ver doc del método en la interface).
// El lock FOR UPDATE carga la fila fresca de competencia: comp puede venir
// obsoleto, el límite y la instancia compartida se resuelven con el snapshot
// del tx (locked). StartDate sale de gametime (relación con las ventanas de
// feedback, misma convención del intake).
func (d *competitionDao) CreateAttemptSession(ctx *gin.Context, comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
	var created *dbs.RunnerSession
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		var locked dbs.Competition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", comp.ID).
			First(&locked).Error; err != nil {
			return fmt.Errorf("error locking competition for attempt: %w", err)
		}
		var count int64
		if err := tx.Model(&dbs.RunnerSession{}).
			Where("competition_id = ? AND athlete_user_id = ?", locked.ID, athleteUserID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("error counting competition attempts: %w", err)
		}
		if locked.MaxAttempts != nil && count >= *locked.MaxAttempts {
			return ErrCompetitionAttemptLimit
		}
		var instanceID int64
		if count == 0 {
			if locked.RouteSessionInstanceID == nil {
				return fmt.Errorf("la competencia %d no tiene instancia de ruta asignada", locked.ID)
			}
			instanceID = *locked.RouteSessionInstanceID
		} else {
			instance, err := createSyntheticRouteInstance(tx, fmt.Sprintf("Competencia ruta %d (intento %d)", locked.ID, count+1))
			if err != nil {
				return err
			}
			instanceID = instance.ID
		}
		rs := &dbs.RunnerSession{
			SessionInstanceID: instanceID,
			AthleteUserID:     athleteUserID,
			Status:            runnersession.RunnerSessionStatusWip,
			StartDate:         gametime.Now(),
			CompetitionID:     &locked.ID,
		}
		var res *gorm.DB
		if count == 0 {
			res = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "session_instance_id"}, {Name: "athlete_user_id"}},
				DoNothing: true,
			}).Create(rs)
		} else {
			res = tx.Create(rs)
		}
		if res.Error != nil {
			return fmt.Errorf("error creating competition attempt session: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrCompetitionAttemptConflict
		}
		created = rs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (d *competitionDao) Approve(ctx *gin.Context, competitionID, approvedByUserID int64) (bool, error) {
	res := d.DB.Model(&dbs.Competition{}).
		Where("id = ? AND status = ?", competitionID, competition.StatusPendingApproval).
		Updates(map[string]interface{}{
			"status":              competition.StatusActive,
			"approved_by_user_id": approvedByUserID,
		})
	if res.Error != nil {
		return false, fmt.Errorf("error approving competition: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (d *competitionDao) Reject(ctx *gin.Context, competitionID int64, reason *string) (bool, error) {
	res := d.DB.Model(&dbs.Competition{}).
		Where("id = ? AND status = ?", competitionID, competition.StatusPendingApproval).
		Updates(map[string]interface{}{
			"status":           competition.StatusRejected,
			"rejection_reason": reason,
		})
	if res.Error != nil {
		return false, fmt.Errorf("error rejecting competition: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// Finish persiste en una sola transacción la transición de status (con guard
// en el WHERE sobre active) y los efectos del cierre: winners, stamps y
// unseen. Si el update no toca filas (carrera: la competencia ya no está
// active) no escribe efectos y devuelve false — ni dobles winners ni dobles
// stamps por doble close.
func (d *competitionDao) Finish(ctx *gin.Context, competitionID int64, finishedAt time.Time, winners []int64, stamps []dbs.GamificationStamp, unseen []dbs.GamificationUnseen) (bool, error) {
	var updated bool
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&dbs.Competition{}).
			Where("id = ? AND status = ?", competitionID, competition.StatusActive).
			Updates(map[string]interface{}{
				"status":      competition.StatusFinished,
				"finished_at": finishedAt,
			})
		if res.Error != nil {
			return fmt.Errorf("error finishing competition: %w", res.Error)
		}
		if res.RowsAffected != 1 {
			return nil
		}
		updated = true
		if len(winners) > 0 {
			rows := make([]dbs.CompetitionWinner, 0, len(winners))
			for _, uid := range winners {
				rows = append(rows, dbs.CompetitionWinner{CompetitionID: competitionID, UserID: uid})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return fmt.Errorf("error creating competition winners: %w", err)
			}
		}
		if len(stamps) > 0 {
			if err := tx.Create(&stamps).Error; err != nil {
				return fmt.Errorf("error creating competition stamps: %w", err)
			}
		}
		if len(unseen) > 0 {
			if err := tx.Create(&unseen).Error; err != nil {
				return fmt.Errorf("error creating competition unseen: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

// Delete borra físicamente la competencia (DELETE cancel D9). El guard de
// status está en el WHERE del DELETE: si tras el FindByID del service la
// competencia dejó de ser pending_approval (owner aprobó interleavado), no
// toca ninguna fila y devuelve false — el service lo mapea a 400, igual que
// Approve/Reject manejan 0 filas. Al borrar limpia en la MISMA transacción
// la instancia sintética route (session/exercise + link): solo-pending no
// tuvo intentos, la limpieza no deja ni deja de borrar nada ajeno.
func (d *competitionDao) Delete(ctx *gin.Context, competitionID int64) (bool, error) {
	var deleted bool
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		var comp dbs.Competition
		// El route_session_instance_id se lee antes del delete para saber
		// qué instancias limpiar (después ya no existe la fila).
		err := tx.Select("route_session_instance_id").
			Where("id = ?", competitionID).
			First(&comp).Error
		if err != nil {
			if isGormNotFound(err) {
				return nil
			}
			return fmt.Errorf("error finding competition to delete: %w", err)
		}
		res := tx.Where("id = ? AND status = ?", competitionID, competition.StatusPendingApproval).
			Delete(&dbs.Competition{})
		if res.Error != nil {
			return fmt.Errorf("error deleting competition: %w", res.Error)
		}
		if res.RowsAffected != 1 {
			return nil
		}
		deleted = true
		if err := tx.Where("competition_id = ?", competitionID).Delete(&dbs.CompetitionParticipant{}).Error; err != nil {
			return fmt.Errorf("error deleting competition participants: %w", err)
		}
		if comp.RouteSessionInstanceID != nil {
			if err := deleteSyntheticRouteInstance(tx, *comp.RouteSessionInstanceID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}

// deleteSyntheticRouteInstance limpia las piezas que CreateWithInstance creó
// para el intento de una competencia route cancelada: links, exercise
// instances y session instance. Corre tras la baja de competencia+participantes
// en la MISMA tx.
func deleteSyntheticRouteInstance(tx *gorm.DB, instanceID int64) error {
	var exerciseIDs []int64
	err := tx.Model(&dbs.SessionExerciseInstance{}).
		Where("session_instance_id = ?", instanceID).
		Pluck("exercise_instance_id", &exerciseIDs).Error
	if err != nil {
		return fmt.Errorf("error plucking competition instance exercises: %w", err)
	}
	if err := tx.Where("session_instance_id = ?", instanceID).Delete(&dbs.SessionExerciseInstance{}).Error; err != nil {
		return fmt.Errorf("error deleting competition instance exercise links: %w", err)
	}
	if len(exerciseIDs) > 0 {
		if err := tx.Where("id IN ?", exerciseIDs).Delete(&dbs.ExerciseInstance{}).Error; err != nil {
			return fmt.Errorf("error deleting competition exercise instances: %w", err)
		}
	}
	if err := tx.Where("id = ?", instanceID).Delete(&dbs.SessionInstance{}).Error; err != nil {
		return fmt.Errorf("error deleting competition session instance: %w", err)
	}
	return nil
}

func (d *competitionDao) HasUnseenTeam(ctx *gin.Context, userID, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.GamificationUnseen{}).
		Where("user_id = ? AND section = ? AND team_id = ?", userID, constants.GamificationSectionCompetitions, teamID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition unseen: %w", err)
	}
	return count > 0, nil
}

// AggregateFeedbackWindow acumula las métricas fuente por atleta dentro de la
// ventana. La ventana de session_date es de fechas: se pasa medianoche local
// de ambos bordes (inclusive) — el granularity de workout_feedback es date.
func (d *competitionDao) AggregateFeedbackWindow(ctx *gin.Context, teamID int64, participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
	if len(participantIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionMetricAgg
	err := d.DB.Table("workout_feedback AS wf").
		Select(`wf.athlete_user_id AS user_id,
			SUM(wf.distance_meters) AS sum_distance,
			SUM(wf.duration_ms) AS sum_duration_ms,
			MAX(wf.peak_speed_kph) AS max_peak`).
		Where("wf.deleted_at IS NULL AND wf.team_id = ? AND wf.athlete_user_id IN ?", teamID, participantIDs).
		Where("wf.session_date >= ? AND wf.session_date <= ?", startAt, endAt).
		Group("wf.athlete_user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error aggregating competition feedback window: %w", err)
	}
	return rows, nil
}

// ListAttemptSessions devuelve los runner sessions tagueados con la
// competencia (intentos registrados, cualquier status): el service los filtra
// finished para el matching y usa todos para los canales WS del cierre.
func (d *competitionDao) ListAttemptSessions(ctx *gin.Context, competitionID int64) ([]dbs.RunnerSession, error) {
	var sessions []dbs.RunnerSession
	err := d.DB.Where("competition_id = ?", competitionID).Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition attempts: %w", err)
	}
	return sessions, nil
}

// ListAttemptsByCompetitionAndAthlete lista los intentos de un atleta
// (cualquier status) con ORDER BY id ASC: el id es cronológico, así que el
// intento 1 queda primero.
func (d *competitionDao) ListAttemptsByCompetitionAndAthlete(ctx *gin.Context, competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
	var sessions []dbs.RunnerSession
	err := d.DB.Where("competition_id = ? AND athlete_user_id = ?", competitionID, athleteUserID).
		Order("id ASC").Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition attempts by athlete: %w", err)
	}
	return sessions, nil
}

// SumFeedbackDistanceByInstances acumula la distancia del feedback ACTIVO del
// atleta por instancia (soft-deleted no suma): instancia sin feedback no
// aparece en la respuesta (el service la deja null).
func (d *competitionDao) SumFeedbackDistanceByInstances(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.CompetitionInstanceDistanceAgg, error) {
	if len(sessionInstanceIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionInstanceDistanceAgg
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Select("assigned_session_id, SUM(distance_meters) AS sum_distance").
		Where("assigned_session_id IN ? AND athlete_user_id = ? AND deleted_at IS NULL", sessionInstanceIDs, athleteUserID).
		Group("assigned_session_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error summing competition feedback distance: %w", err)
	}
	return rows, nil
}

// ListAttemptPoints une en una query los puntos GPS de los intentos (los
// puntos viven del feedback activo del atleta sobre la misma instancia).
func (d *competitionDao) ListAttemptPoints(ctx *gin.Context, sessionInstanceIDs, athleteUserIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
	if len(sessionInstanceIDs) == 0 || len(athleteUserIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionAttemptPoint
	err := d.DB.Table("workout_feedback_points AS p").
		Select("p.session_instance_id, wf.athlete_user_id AS athlete_user_id, p.latitude, p.longitude").
		Joins("JOIN workout_feedback wf ON wf.id = p.feedback_id AND wf.deleted_at IS NULL").
		Where("p.session_instance_id IN ? AND wf.athlete_user_id IN ?", sessionInstanceIDs, athleteUserIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition attempt points: %w", err)
	}
	return rows, nil
}

// CountAttemptsByCompetitionAndAthlete devuelve competencia→cantidad de
// intentos del atleta, una sola query batch para el listado y el detalle
// (Gap 36 D7). El LEFT JOIN con competition_participant distingue al
// participante sin intentos (fila con count 0) del no-participante (no hay
// fila): attempts_used null solo tiene sentido en el segundo caso.
func (d *competitionDao) CountAttemptsByCompetitionAndAthlete(ctx *gin.Context, competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
	if len(competitionIDs) == 0 {
		return map[int64]int64{}, nil
	}
	var rows []dbs.CompetitionAttemptCountAgg
	err := d.DB.Table("competition_participant AS cp").
		Select("cp.competition_id AS competition_id, COUNT(rs.id) AS attempt_count").
		Joins("LEFT JOIN runner_session rs ON rs.competition_id = cp.competition_id AND rs.athlete_user_id = cp.user_id").
		Where("cp.user_id = ? AND cp.competition_id IN ?", athleteUserID, competitionIDs).
		Group("cp.competition_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error counting competition attempts by athlete: %w", err)
	}
	counts := make(map[int64]int64, len(rows))
	for _, r := range rows {
		counts[r.CompetitionID] = r.AttemptCount
	}
	return counts, nil
}
