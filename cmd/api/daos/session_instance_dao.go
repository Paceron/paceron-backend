package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// SessionInstanceDaoInterface: las instancias son inmutables (design.md D1) —
// solo Create/FindByID/Delete físico, sin Update ni SoftDelete.
type SessionInstanceDaoInterface interface {
	Create(ctx *gin.Context, s *dbs.SessionInstance) error
	// CreateBatch inserta N instancias en un solo INSERT con RETURNING —
	// gorm rellena cada row.ID por posición (instanciación batch, design.md
	// D6). Lote vacío no toca la DB.
	CreateBatch(ctx *gin.Context, rows []*dbs.SessionInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.SessionInstance, error)
	// FindByIDs trae varias instancias en una sola consulta — evita el N+1
	// al armar la vista de calendario por lotes.
	FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.SessionInstance, error)
	Delete(ctx *gin.Context, id int64) error
	// DeleteByIDs borra físico varias instancias en un solo DELETE con IN.
	DeleteByIDs(ctx *gin.Context, ids []int64) error
	// HasFeedback reporta si algún workout_feedback activo referencia esta
	// instancia vía assigned_session_id (FK opaca, design.md D3).
	HasFeedback(ctx *gin.Context, id int64) (bool, error)
	// HasFeedbackByIDs devuelve los ids indicados que tienen algún
	// workout_feedback activo (deleted_at IS NULL), espejo en lote de
	// HasFeedback (design.md D6c). Lote vacío no toca la DB.
	HasFeedbackByIDs(ctx *gin.Context, ids []int64) ([]int64, error)
	// HasInstanceAccess aplica la regla dual de session-instance-detail D2:
	// día con esta instancia donde el caller es miembro activo del grupo u
	// owner del equipo, O feedback activo sobre la instancia donde el caller
	// es atleta, reportante u owner del equipo. TERCERA rama
	// (competencias-ruta-intake D8): si la instancia es la sintética de una
	// competencia type=route, autoriza al participante de esa competencia sin
	// filtro de status (el front suscribe el canal session:{id} del intento en
	// el pre-start, antes de que exista feedback).
	HasInstanceAccess(ctx *gin.Context, instanceID, callerID int64) (bool, error)
	// HasInstanceAccessBatch: espejo en lote de HasInstanceAccess (misma regla,
	// incluida la rama de competencias route). Cantidad de queries independiente
	// de la lista; todo userIDs queda en el map (false si sin acceso). Lote
	// vacío no toca la DB.
	HasInstanceAccessBatch(ctx *gin.Context, instanceID int64, userIDs []int64) (map[int64]bool, error)
}

type sessionInstanceDao struct {
	DB *gorm.DB
}

func NewSessionInstanceDao(db *gorm.DB) SessionInstanceDaoInterface {
	return &sessionInstanceDao{DB: db}
}

func (d *sessionInstanceDao) Create(ctx *gin.Context, s *dbs.SessionInstance) error {
	if err := d.DB.Create(s).Error; err != nil {
		return fmt.Errorf("error creating session instance: %w", err)
	}
	return nil
}

func (d *sessionInstanceDao) CreateBatch(ctx *gin.Context, rows []*dbs.SessionInstance) error {
	if len(rows) == 0 {
		return nil
	}
	if err := d.DB.Create(&rows).Error; err != nil {
		return fmt.Errorf("error creating session instances batch: %w", err)
	}
	return nil
}

func (d *sessionInstanceDao) FindByID(ctx *gin.Context, id int64) (*dbs.SessionInstance, error) {
	var s dbs.SessionInstance
	err := d.DB.Where("id = ?", id).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding session instance: %w", err)
	}
	return &s, nil
}

func (d *sessionInstanceDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.SessionInstance, error) {
	var rows []dbs.SessionInstance
	if len(ids) == 0 {
		return rows, nil
	}
	err := d.DB.Where("id IN ?", ids).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding session instances by ids: %w", err)
	}
	return rows, nil
}

func (d *sessionInstanceDao) Delete(ctx *gin.Context, id int64) error {
	if err := d.DB.Delete(&dbs.SessionInstance{}, id).Error; err != nil {
		return fmt.Errorf("error deleting session instance: %w", err)
	}
	return nil
}

func (d *sessionInstanceDao) DeleteByIDs(ctx *gin.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := d.DB.Where("id IN ?", ids).Delete(&dbs.SessionInstance{}).Error; err != nil {
		return fmt.Errorf("error deleting session instances batch: %w", err)
	}
	return nil
}

func (d *sessionInstanceDao) HasFeedbackByIDs(ctx *gin.Context, ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id IN ? AND deleted_at IS NULL", ids).
		Distinct().Pluck("assigned_session_id", &out).Error
	if err != nil {
		return nil, fmt.Errorf("error checking session instances feedback: %w", err)
	}
	return out, nil
}

func (d *sessionInstanceDao) HasFeedback(ctx *gin.Context, id int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id = ? AND deleted_at IS NULL", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking session instance feedback: %w", err)
	}
	return count > 0, nil
}

func (d *sessionInstanceDao) HasInstanceAccess(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
	var dayCount int64
	err := d.DB.Model(&dbs.GroupCalendarDay{}).
		Joins("JOIN groups g ON g.id = group_calendar_days.group_id AND g.deleted_at IS NULL").
		Joins("JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL").
		Where("group_calendar_days.session_instance_id = ? AND (t.owner_id = ? OR EXISTS (SELECT 1 FROM group_users gu WHERE gu.group_id = g.id AND gu.user_id = ? AND gu.deleted_at IS NULL AND (gu.date_end IS NULL OR gu.date_end > NOW())))", instanceID, callerID, callerID).
		Count(&dayCount).Error
	if err != nil {
		return false, fmt.Errorf("error checking session instance day access: %w", err)
	}
	if dayCount > 0 {
		return true, nil
	}
	var fbCount int64
	err = d.DB.Model(&dbs.WorkoutFeedback{}).
		Joins("LEFT JOIN teams t ON t.id = workout_feedback.team_id").
		Where("workout_feedback.assigned_session_id = ? AND workout_feedback.deleted_at IS NULL AND (workout_feedback.athlete_user_id = ? OR workout_feedback.feedback_owner_user_id = ? OR t.owner_id = ?)", instanceID, callerID, callerID, callerID).
		Count(&fbCount).Error
	if err != nil {
		return false, fmt.Errorf("error checking session instance feedback access: %w", err)
	}
	if fbCount > 0 {
		return true, nil
	}
	var compCount int64
	err = d.DB.Model(&dbs.Competition{}).
		Joins("JOIN competition_participant ON competition_participant.competition_id = competition.id AND competition_participant.user_id = ?", callerID).
		Where("competition.route_session_instance_id = ? AND competition.type = ?", instanceID, competition.TypeRoute).
		Count(&compCount).Error
	if err != nil {
		return false, fmt.Errorf("error checking session instance competition access: %w", err)
	}
	return compCount > 0, nil
}

// hasAccessBatchSQL: misma condición que HasInstanceAccess, lista via unnest.
const hasAccessBatchDaySQL = `
SELECT u.user_id
FROM unnest($1::bigint[]) AS u(user_id)
WHERE EXISTS (
    SELECT 1
    FROM group_calendar_days
    JOIN groups g ON g.id = group_calendar_days.group_id AND g.deleted_at IS NULL
    JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL
    WHERE group_calendar_days.session_instance_id = $2
      AND (t.owner_id = u.user_id
        OR EXISTS (
            SELECT 1 FROM group_users gu
            WHERE gu.group_id = g.id
              AND gu.user_id = u.user_id
              AND gu.deleted_at IS NULL
              AND (gu.date_end IS NULL OR gu.date_end > NOW())
        ))
)`

const hasAccessBatchFeedbackSQL = `
SELECT u.user_id
FROM unnest($1::bigint[]) AS u(user_id)
WHERE EXISTS (
    SELECT 1
    FROM workout_feedback
    LEFT JOIN teams t ON t.id = workout_feedback.team_id
    WHERE workout_feedback.assigned_session_id = $2
      AND workout_feedback.deleted_at IS NULL
      AND (workout_feedback.athlete_user_id = u.user_id
        OR workout_feedback.feedback_owner_user_id = u.user_id
        OR t.owner_id = u.user_id)
)`

const hasAccessBatchCompetitionSQL = `
SELECT u.user_id
FROM unnest($1::bigint[]) AS u(user_id)
WHERE EXISTS (
    SELECT 1
    FROM competition c
    JOIN competition_participant cp ON cp.competition_id = c.id AND cp.user_id = u.user_id
    WHERE c.route_session_instance_id = $2
      AND c.type = $3
)`

// batchAccessIDs corre una query EXISTS sobre los usuarios pendientes.
func (d *sessionInstanceDao) batchAccessIDs(query string, pending []int64, args ...any) ([]int64, error) {
	authorized := []int64{}
	all := append([]any{pending}, args...)
	err := d.DB.Raw(query, all...).Scan(&authorized).Error
	if err != nil {
		return nil, fmt.Errorf("error checking session instance access batch: %w", err)
	}
	return authorized, nil
}

// remainingPending: usuarios de pending no presentes en authorized.
func remainingPending(pending, authorized []int64) []int64 {
	if len(authorized) == 0 {
		return pending
	}
	authSet := make(map[int64]struct{}, len(authorized))
	for _, id := range authorized {
		authSet[id] = struct{}{}
	}
	out := make([]int64, 0, len(pending)-len(authorized))
	for _, id := range pending {
		if _, ok := authSet[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

func (d *sessionInstanceDao) HasInstanceAccessBatch(ctx *gin.Context, instanceID int64, userIDs []int64) (map[int64]bool, error) {
	access := make(map[int64]bool, len(userIDs))
	if len(userIDs) == 0 {
		return access, nil
	}
	pending := make([]int64, len(userIDs))
	copy(pending, userIDs)
	// Todo el lote arranca en false: solo las ramas autorizadas lo pisan.
	for _, id := range pending {
		access[id] = false
	}

	authorized, err := d.batchAccessIDs(hasAccessBatchDaySQL, pending, instanceID)
	if err != nil {
		return nil, err
	}
	for _, id := range authorized {
		access[id] = true
	}
	pending = remainingPending(pending, authorized)
	if len(pending) == 0 {
		return access, nil
	}

	authorized, err = d.batchAccessIDs(hasAccessBatchFeedbackSQL, pending, instanceID)
	if err != nil {
		return nil, err
	}
	for _, id := range authorized {
		access[id] = true
	}
	pending = remainingPending(pending, authorized)
	if len(pending) == 0 {
		return access, nil
	}

	authorized, err = d.batchAccessIDs(hasAccessBatchCompetitionSQL, pending, instanceID, competition.TypeRoute)
	if err != nil {
		return nil, err
	}
	for _, id := range authorized {
		access[id] = true
	}
	return access, nil
}
