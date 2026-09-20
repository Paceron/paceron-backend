package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// ExerciseInstanceDaoInterface: las instancias son inmutables (design.md D1) —
// solo Create/FindByID/Delete físico, sin Update ni SoftDelete.
type ExerciseInstanceDaoInterface interface {
	Create(ctx *gin.Context, e *dbs.ExerciseInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.ExerciseInstance, error)
	Delete(ctx *gin.Context, id int64) error
	// HasFeedback reporta si algún workout_feedback activo referencia esta
	// instancia vía assigned_exercise_id (FK opaca, design.md D3).
	HasFeedback(ctx *gin.Context, id int64) (bool, error)
}

type exerciseInstanceDao struct {
	DB *gorm.DB
}

func NewExerciseInstanceDao(db *gorm.DB) ExerciseInstanceDaoInterface {
	return &exerciseInstanceDao{DB: db}
}

func (d *exerciseInstanceDao) Create(ctx *gin.Context, e *dbs.ExerciseInstance) error {
	if err := d.DB.Create(e).Error; err != nil {
		return fmt.Errorf("error creating exercise instance: %w", err)
	}
	return nil
}

func (d *exerciseInstanceDao) FindByID(ctx *gin.Context, id int64) (*dbs.ExerciseInstance, error) {
	var e dbs.ExerciseInstance
	err := d.DB.Where("id = ?", id).First(&e).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding exercise instance: %w", err)
	}
	return &e, nil
}

func (d *exerciseInstanceDao) Delete(ctx *gin.Context, id int64) error {
	if err := d.DB.Delete(&dbs.ExerciseInstance{}, id).Error; err != nil {
		return fmt.Errorf("error deleting exercise instance: %w", err)
	}
	return nil
}

func (d *exerciseInstanceDao) HasFeedback(ctx *gin.Context, id int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_exercise_id = ? AND deleted_at IS NULL", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking exercise instance feedback: %w", err)
	}
	return count > 0, nil
}
