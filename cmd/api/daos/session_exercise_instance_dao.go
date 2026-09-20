package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// SessionExerciseInstanceDaoInterface: los links son inmutables (design.md D1)
// y viven y mueren con su SessionInstance — sin Update ni SoftDelete.
type SessionExerciseInstanceDaoInterface interface {
	Create(ctx *gin.Context, se *dbs.SessionExerciseInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.SessionExerciseInstance, error)
	FindBySessionInstance(ctx *gin.Context, sessionInstanceID int64) ([]dbs.SessionExerciseInstance, error)
	Delete(ctx *gin.Context, id int64) error
	// DeleteBySessionInstance borra físico todos los links de una
	// SessionInstance — paso hijo-antes-que-padre del borrado de instancia
	// superada (design.md D10).
	DeleteBySessionInstance(ctx *gin.Context, sessionInstanceID int64) error
}

type sessionExerciseInstanceDao struct {
	DB *gorm.DB
}

func NewSessionExerciseInstanceDao(db *gorm.DB) SessionExerciseInstanceDaoInterface {
	return &sessionExerciseInstanceDao{DB: db}
}

func (d *sessionExerciseInstanceDao) Create(ctx *gin.Context, se *dbs.SessionExerciseInstance) error {
	if err := d.DB.Create(se).Error; err != nil {
		return fmt.Errorf("error creating session exercise instance: %w", err)
	}
	return nil
}

func (d *sessionExerciseInstanceDao) FindByID(ctx *gin.Context, id int64) (*dbs.SessionExerciseInstance, error) {
	var se dbs.SessionExerciseInstance
	err := d.DB.Where("id = ?", id).First(&se).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding session exercise instance: %w", err)
	}
	return &se, nil
}

func (d *sessionExerciseInstanceDao) FindBySessionInstance(ctx *gin.Context, sessionInstanceID int64) ([]dbs.SessionExerciseInstance, error) {
	var rows []dbs.SessionExerciseInstance
	err := d.DB.Where("session_instance_id = ?", sessionInstanceID).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing session exercise instances: %w", err)
	}
	return rows, nil
}

func (d *sessionExerciseInstanceDao) Delete(ctx *gin.Context, id int64) error {
	if err := d.DB.Delete(&dbs.SessionExerciseInstance{}, id).Error; err != nil {
		return fmt.Errorf("error deleting session exercise instance: %w", err)
	}
	return nil
}

func (d *sessionExerciseInstanceDao) DeleteBySessionInstance(ctx *gin.Context, sessionInstanceID int64) error {
	err := d.DB.Where("session_instance_id = ?", sessionInstanceID).Delete(&dbs.SessionExerciseInstance{}).Error
	if err != nil {
		return fmt.Errorf("error deleting session exercise instances: %w", err)
	}
	return nil
}
