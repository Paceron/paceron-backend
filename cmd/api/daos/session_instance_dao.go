package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// SessionInstanceDaoInterface: las instancias son inmutables (design.md D1) —
// solo Create/FindByID/Delete físico, sin Update ni SoftDelete.
type SessionInstanceDaoInterface interface {
	Create(ctx *gin.Context, s *dbs.SessionInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.SessionInstance, error)
	// FindByIDs trae varias instancias en una sola consulta — evita el N+1
	// al armar la vista de calendario por lotes.
	FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.SessionInstance, error)
	Delete(ctx *gin.Context, id int64) error
	// HasFeedback reporta si algún workout_feedback activo referencia esta
	// instancia vía assigned_session_id (FK opaca, design.md D3).
	HasFeedback(ctx *gin.Context, id int64) (bool, error)
	// HasInstanceAccess aplica la regla dual de session-instance-detail D2:
	// día con esta instancia donde el caller es miembro activo del grupo u
	// owner del equipo, O feedback activo sobre la instancia donde el caller
	// es atleta, reportante u owner del equipo.
	HasInstanceAccess(ctx *gin.Context, instanceID, callerID int64) (bool, error)
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
	return fbCount > 0, nil
}
