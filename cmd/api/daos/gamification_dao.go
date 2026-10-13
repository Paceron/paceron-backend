package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// GamificationDAOInterface reúne el acceso a datos del módulo de gamificación:
// lecturas batch (3 exactas) del cálculo de rachas y el EXISTS de novedades
// embebidas. Escrituras (Triggers/seen) llegan en tasks posteriores.
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
