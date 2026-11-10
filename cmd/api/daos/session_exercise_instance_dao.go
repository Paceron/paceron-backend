package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// SessionExerciseInstanceWithExercise es un link con su ExerciseInstance
// congelado. Exercise es nil cuando el link quedó huérfano (sin fila en
// exercise_instances), para que el mapper conserve el error explícito.
type SessionExerciseInstanceWithExercise struct {
	Link     dbs.SessionExerciseInstance
	Exercise *dbs.ExerciseInstance
}

// SessionExerciseInstanceDaoInterface: los links son inmutables (design.md D1)
// y viven y mueren con su SessionInstance — sin Update ni SoftDelete.
type SessionExerciseInstanceDaoInterface interface {
	Create(ctx *gin.Context, se *dbs.SessionExerciseInstance) error
	// CreateBatch inserta N links en un solo INSERT (instanciación batch,
	// design.md D6; no hace RETURNING: los ids no se usan). Lote vacío no
	// toca la DB.
	CreateBatch(ctx *gin.Context, rows []*dbs.SessionExerciseInstance) error
	FindByID(ctx *gin.Context, id int64) (*dbs.SessionExerciseInstance, error)
	FindBySessionInstance(ctx *gin.Context, sessionInstanceID int64) ([]dbs.SessionExerciseInstance, error)
	// FindBySessionInstances trae los links de varias instancias en una
	// sola consulta — evita el N+1 al armar la vista de calendario por lotes.
	FindBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) ([]dbs.SessionExerciseInstance, error)
	// FindWithExercisesBySessionInstances resuelve links + ejercicios en una
	// sola consulta (LEFT JOIN para detectar huérfanos), ordenados por
	// session_instance_id e id de link para conservar el orden del detalle.
	FindWithExercisesBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) ([]SessionExerciseInstanceWithExercise, error)
	Delete(ctx *gin.Context, id int64) error
	// DeleteBySessionInstance borra físico todos los links de una
	// SessionInstance — paso hijo-antes-que-padre del borrado de instancia
	// superada (design.md D10).
	DeleteBySessionInstance(ctx *gin.Context, sessionInstanceID int64) error
	// DeleteBySessionInstances borra físico los links de varias instancias
	// en un solo DELETE con IN (design.md D6c). Lote vacío no toca la DB.
	DeleteBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) error
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

func (d *sessionExerciseInstanceDao) CreateBatch(ctx *gin.Context, rows []*dbs.SessionExerciseInstance) error {
	if len(rows) == 0 {
		return nil
	}
	if err := d.DB.Create(&rows).Error; err != nil {
		return fmt.Errorf("error creating session exercise instances batch: %w", err)
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

func (d *sessionExerciseInstanceDao) FindBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) ([]dbs.SessionExerciseInstance, error) {
	var rows []dbs.SessionExerciseInstance
	if len(sessionInstanceIDs) == 0 {
		return rows, nil
	}
	err := d.DB.Where("session_instance_id IN ?", sessionInstanceIDs).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding session exercise instances by session instances: %w", err)
	}
	return rows, nil
}

type sessionExerciseInstanceExerciseRow struct {
	LinkID                   int64      `gorm:"column:link_id"`
	SessionInstanceID        int64      `gorm:"column:session_instance_id"`
	ExerciseInstanceID       int64      `gorm:"column:exercise_instance_id"`
	Role                     string     `gorm:"column:role"`
	RepeatCount              int        `gorm:"column:repeat_count"`
	RestMinutes              int        `gorm:"column:rest_minutes"`
	ExerciseID               *int64     `gorm:"column:exercise_id"`
	ExerciseName             *string    `gorm:"column:exercise_name"`
	ExerciseDescription      *string    `gorm:"column:exercise_description"`
	ExerciseKind             *string    `gorm:"column:exercise_kind"`
	ExerciseIntensity        *string    `gorm:"column:exercise_intensity"`
	ExerciseMinutes          *int       `gorm:"column:exercise_minutes"`
	ExerciseDistanceM        *int       `gorm:"column:exercise_distance_m"`
	ExerciseSpeedKph         *float64   `gorm:"column:exercise_speed_kph"`
	ExerciseMuscleGroup      *string    `gorm:"column:exercise_muscle_group"`
	ExerciseVideoURL         *string    `gorm:"column:exercise_video_url"`
	ExerciseSourceExerciseID *int64     `gorm:"column:exercise_source_exercise_id"`
	ExerciseCreatedAt        *time.Time `gorm:"column:exercise_created_at"`
}

func (d *sessionExerciseInstanceDao) FindWithExercisesBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) ([]SessionExerciseInstanceWithExercise, error) {
	rows := make([]SessionExerciseInstanceWithExercise, 0)
	if len(sessionInstanceIDs) == 0 {
		return rows, nil
	}
	var joined []sessionExerciseInstanceExerciseRow
	err := d.DB.Table("session_exercise_instances sei").
		Select(`sei.id AS link_id, sei.session_instance_id, sei.exercise_instance_id, sei.role,
			sei.repeat_count, sei.rest_minutes,
			ei.id AS exercise_id, ei.name AS exercise_name, ei.description AS exercise_description,
			ei.kind AS exercise_kind, ei.intensity AS exercise_intensity, ei.minutes AS exercise_minutes,
			ei.distance_m AS exercise_distance_m, ei.speed_kph AS exercise_speed_kph,
			ei.muscle_group AS exercise_muscle_group, ei.video_url AS exercise_video_url,
			ei.source_exercise_id AS exercise_source_exercise_id, ei.created_at AS exercise_created_at`).
		Joins("LEFT JOIN exercise_instances ei ON ei.id = sei.exercise_instance_id").
		Where("sei.session_instance_id IN ?", sessionInstanceIDs).
		Order("sei.session_instance_id ASC, sei.id ASC").
		Scan(&joined).Error
	if err != nil {
		return nil, fmt.Errorf("error finding session exercise instances with exercises: %w", err)
	}
	for _, row := range joined {
		item := SessionExerciseInstanceWithExercise{
			Link: dbs.SessionExerciseInstance{
				ID:                 row.LinkID,
				SessionInstanceID:  row.SessionInstanceID,
				ExerciseInstanceID: row.ExerciseInstanceID,
				Role:               row.Role,
				RepeatCount:        row.RepeatCount,
				RestMinutes:        row.RestMinutes,
			},
		}
		if row.ExerciseID != nil {
			exercise := &dbs.ExerciseInstance{
				ID:               *row.ExerciseID,
				Description:      row.ExerciseDescription,
				Intensity:        row.ExerciseIntensity,
				Minutes:          row.ExerciseMinutes,
				DistanceM:        row.ExerciseDistanceM,
				SpeedKph:         row.ExerciseSpeedKph,
				MuscleGroup:      row.ExerciseMuscleGroup,
				VideoURL:         row.ExerciseVideoURL,
				SourceExerciseID: row.ExerciseSourceExerciseID,
			}
			if row.ExerciseName != nil {
				exercise.Name = *row.ExerciseName
			}
			if row.ExerciseKind != nil {
				exercise.Kind = *row.ExerciseKind
			}
			if row.ExerciseCreatedAt != nil {
				exercise.CreatedAt = *row.ExerciseCreatedAt
			}
			item.Exercise = exercise
		}
		rows = append(rows, item)
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

func (d *sessionExerciseInstanceDao) DeleteBySessionInstances(ctx *gin.Context, sessionInstanceIDs []int64) error {
	if len(sessionInstanceIDs) == 0 {
		return nil
	}
	err := d.DB.Where("session_instance_id IN ?", sessionInstanceIDs).Delete(&dbs.SessionExerciseInstance{}).Error
	if err != nil {
		return fmt.Errorf("error deleting session exercise instances batch: %w", err)
	}
	return nil
}
