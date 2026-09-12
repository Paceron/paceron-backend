package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type ExerciseDaoInterface interface {
	Create(ctx *gin.Context, e *dbs.Exercise) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error)
	Update(ctx *gin.Context, e *dbs.Exercise) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type exerciseDao struct {
	DB *gorm.DB
}

func NewExerciseDao(database *gorm.DB) ExerciseDaoInterface {
	return &exerciseDao{DB: database}
}

func (d *exerciseDao) Create(ctx *gin.Context, e *dbs.Exercise) error {
	return d.DB.Create(e).Error
}

func (d *exerciseDao) FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
	var e dbs.Exercise
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&e).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding exercise: %w", err)
	}
	return &e, nil
}

func (d *exerciseDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) {
	var exercises []dbs.Exercise
	err := d.DB.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("id").Find(&exercises).Error
	if err != nil {
		return nil, fmt.Errorf("error listing exercises: %w", err)
	}
	return exercises, nil
}

// Update usa un map (no Updates(struct)) para que los punteros nil sí
// limpien la columna a NULL — Updates(struct) de GORM omite campos zero-value.
func (d *exerciseDao) Update(ctx *gin.Context, e *dbs.Exercise) error {
	return d.DB.Model(&dbs.Exercise{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":         e.Name,
		"description":  e.Description,
		"kind":         e.Kind,
		"intensity":    e.Intensity,
		"minutes":      e.Minutes,
		"distance_m":   e.DistanceM,
		"speed_kph":    e.SpeedKph,
		"muscle_group": e.MuscleGroup,
	}).Error
}

func (d *exerciseDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Exercise{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}
