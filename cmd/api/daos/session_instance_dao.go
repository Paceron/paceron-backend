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
	Delete(ctx *gin.Context, id int64) error
	// HasFeedback reporta si algún workout_feedback activo referencia esta
	// instancia vía assigned_session_id (FK opaca, design.md D3).
	HasFeedback(ctx *gin.Context, id int64) (bool, error)
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
