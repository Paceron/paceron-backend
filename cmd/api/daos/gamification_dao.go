package daos

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// GamificationDAOInterface reúne el acceso a datos del módulo de gamificación:
// lecturas batch (3 exactas) del cálculo de rachas, el EXISTS de novedades
// embebidas y las escrituras del POST seen + trigger de racha (Task 2).
type GamificationDAOInterface interface {
	// FindActiveGroupMemberships devuelve las membresías activas del usuario
	// (mismo criterio de FindByUserID del groupUserDao: sin deleted y con
	// ventana de membresía vigente).
	FindActiveGroupMemberships(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	// FindTrainingDaysUntil devuelve los días kind='training' de los grupos
	// dados hasta la fecha provista (inclusive) — rango ABIERTO hacia el
	// pasado a propósito: best de racha se computa sobre TODO el histórico
	// hasta ayer.
	FindTrainingDaysUntil(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error)
	// FindFinishedRunnerSessions devuelve las runner sessions finished del
	// atleta sobre las instancias indicadas (batch por IN, sin N+1).
	FindFinishedRunnerSessions(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.RunnerSession, error)
	// HasUnseenSection indica si hay filas de gamification_unseen para
	// (usuario, sección).
	HasUnseenSection(ctx *gin.Context, userID int64, section string) (bool, error)
	// DeleteUnseen borra las unseen del usuario para la sección (POST seen):
	// teamID filtra SOLO competitions; nil limpia todas las de la sección,
	// incluidas las por-equipo. Devuelve la cantidad borrada (0 = no-op).
	DeleteUnseen(ctx *gin.Context, userID int64, section string, teamID *int64) (int64, error)
	// InsertUnseen inserta filas unseen en un solo batch (trigger de racha:
	// hasta 2 filas por mutación, una por racha afectada).
	InsertUnseen(ctx *gin.Context, unseen []dbs.GamificationUnseen) error
	// GetStreakState devuelve el estado persistido de rachas del usuario
	// (nil si no tiene fila todavía; baseline "antes" del trigger D3).
	GetStreakState(ctx *gin.Context, userID int64) (*dbs.GamificationStreakState, error)
	// UpsertStreakState persiste los currents post-comparación del trigger.
	UpsertStreakState(ctx *gin.Context, userID int64, strictCurrent, flexibleCurrent int) error
}

type gamificationDao struct {
	DB *gorm.DB
}

func NewGamificationDao(database *gorm.DB) GamificationDAOInterface {
	return &gamificationDao{DB: database}
}

func (d *gamificationDao) FindActiveGroupMemberships(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	var memberships []dbs.GroupUser
	err := d.DB.
		Where("user_id = ? AND deleted_at IS NULL AND (date_end IS NULL OR date_end > NOW())", userID).
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("error finding gamification memberships: %w", err)
	}
	return memberships, nil
}

func (d *gamificationDao) FindTrainingDaysUntil(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var days []dbs.GroupCalendarDay
	err := d.DB.
		Where("group_id IN ? AND date <= ? AND kind = ?", groupIDs, until, string(constants.GroupCalendarDayKindTraining)).
		Order("date").
		Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error finding gamification training days: %w", err)
	}
	return days, nil
}

func (d *gamificationDao) FindFinishedRunnerSessions(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.RunnerSession, error) {
	if len(sessionInstanceIDs) == 0 {
		return nil, nil
	}
	var sessions []dbs.RunnerSession
	err := d.DB.
		Where("session_instance_id IN ? AND athlete_user_id = ? AND status = ?",
			sessionInstanceIDs, athleteUserID, runnersession.RunnerSessionStatusFinished).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error finding gamification runner sessions: %w", err)
	}
	return sessions, nil
}

func (d *gamificationDao) HasUnseenSection(ctx *gin.Context, userID int64, section string) (bool, error) {
	var n int64
	err := d.DB.Model(&dbs.GamificationUnseen{}).
		Where("user_id = ? AND section = ?", userID, section).
		Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("error checking gamification unseen: %w", err)
	}
	return n > 0, nil
}

func (d *gamificationDao) DeleteUnseen(ctx *gin.Context, userID int64, section string, teamID *int64) (int64, error) {
	q := d.DB.Where("user_id = ? AND section = ?", userID, section)
	if teamID != nil {
		q = q.Where("team_id = ?", *teamID)
	}
	res := q.Delete(&dbs.GamificationUnseen{})
	if res.Error != nil {
		return 0, fmt.Errorf("error deleting gamification unseen: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (d *gamificationDao) InsertUnseen(ctx *gin.Context, unseen []dbs.GamificationUnseen) error {
	if len(unseen) == 0 {
		return nil
	}
	err := d.DB.Create(&unseen).Error
	if err != nil {
		return fmt.Errorf("error inserting gamification unseen: %w", err)
	}
	return nil
}

func (d *gamificationDao) GetStreakState(ctx *gin.Context, userID int64) (*dbs.GamificationStreakState, error) {
	var state dbs.GamificationStreakState
	err := d.DB.Where("user_id = ?", userID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting gamification streak state: %w", err)
	}
	return &state, nil
}

func (d *gamificationDao) UpsertStreakState(ctx *gin.Context, userID int64, strictCurrent, flexibleCurrent int) error {
	state := dbs.GamificationStreakState{
		UserID:          userID,
		StrictCurrent:   strictCurrent,
		FlexibleCurrent: flexibleCurrent,
	}
	err := d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"strict_current", "flexible_current", "updated_at"}),
	}).Create(&state).Error
	if err != nil {
		return fmt.Errorf("error upserting gamification streak state: %w", err)
	}
	return nil
}
