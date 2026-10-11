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
	// CreateBatch inserta N ejercicios de instancia en un solo INSERT con
	// RETURNING — gorm rellena cada row.ID por posición (design.md D6).
	// Lote vacío no toca la DB.
	CreateBatch(ctx *gin.Context, rows []*dbs.ExerciseInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.ExerciseInstance, error)
	// FindByIDs trae varios ejercicios de instancia en una sola consulta —
	// evita el N+1 al armar el detalle D9 de una SessionInstance (los links
	// vienen de FindBySessionInstance y resuelven sus ejercicios por lote).
	FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.ExerciseInstance, error)
	Delete(ctx *gin.Context, id int64) error
	// DeleteByIDs borra físico varios ejercicios de instancia en un solo
	// DELETE con IN. Lote vacío no toca la DB.
	DeleteByIDs(ctx *gin.Context, ids []int64) error
	// HasFeedback reporta si algún workout_feedback activo referencia esta
	// instancia vía assigned_exercise_id (FK opaca, design.md D3).
	HasFeedback(ctx *gin.Context, id int64) (bool, error)
	// HasFeedbackByIDs devuelve los ids indicados que tienen algún
	// workout_feedback activo (deleted_at IS NULL), espejo en lote de
	// HasFeedback (design.md D6c). Lote vacío no toca la DB.
	HasFeedbackByIDs(ctx *gin.Context, ids []int64) ([]int64, error)
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

func (d *exerciseInstanceDao) CreateBatch(ctx *gin.Context, rows []*dbs.ExerciseInstance) error {
	if len(rows) == 0 {
		return nil
	}
	if err := d.DB.Create(&rows).Error; err != nil {
		return fmt.Errorf("error creating exercise instances batch: %w", err)
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

func (d *exerciseInstanceDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.ExerciseInstance, error) {
	var rows []dbs.ExerciseInstance
	if len(ids) == 0 {
		return rows, nil
	}
	err := d.DB.Where("id IN ?", ids).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding exercise instances by ids: %w", err)
	}
	return rows, nil
}

func (d *exerciseInstanceDao) Delete(ctx *gin.Context, id int64) error {
	if err := d.DB.Delete(&dbs.ExerciseInstance{}, id).Error; err != nil {
		return fmt.Errorf("error deleting exercise instance: %w", err)
	}
	return nil
}

func (d *exerciseInstanceDao) DeleteByIDs(ctx *gin.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := d.DB.Where("id IN ?", ids).Delete(&dbs.ExerciseInstance{}).Error; err != nil {
		return fmt.Errorf("error deleting exercise instances batch: %w", err)
	}
	return nil
}

func (d *exerciseInstanceDao) HasFeedbackByIDs(ctx *gin.Context, ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_exercise_id IN ? AND deleted_at IS NULL", ids).
		Distinct().Pluck("assigned_exercise_id", &out).Error
	if err != nil {
		return nil, fmt.Errorf("error checking exercise instances feedback: %w", err)
	}
	return out, nil
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
