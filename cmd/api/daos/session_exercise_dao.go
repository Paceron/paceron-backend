package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type SessionExerciseDaoInterface interface {
	FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error)
	// FindBySessions son los links de varias sesiones de catálogo en una
	// sola query, ordenados por (session_id, id) — misma forma que 1
	// FindBySession por sesión (instanciación batch, design.md D6).
	FindBySessions(ctx *gin.Context, sessionIDs []int64) ([]dbs.SessionExercise, error)
	ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error
}

type sessionExerciseDao struct {
	DB *gorm.DB
}

func NewSessionExerciseDao(database *gorm.DB) SessionExerciseDaoInterface {
	return &sessionExerciseDao{DB: database}
}

func (d *sessionExerciseDao) FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
	var rows []dbs.SessionExercise
	err := d.DB.Where("session_id = ?", sessionID).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing session exercises: %w", err)
	}
	return rows, nil
}

func (d *sessionExerciseDao) FindBySessions(ctx *gin.Context, sessionIDs []int64) ([]dbs.SessionExercise, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.SessionExercise
	err := d.DB.Where("session_id IN ?", sessionIDs).Order("session_id, id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing session exercises by sessions: %w", err)
	}
	return rows, nil
}

// ReplaceForSession borra el set anterior y crea el nuevo en una
// transacción — el PUT de Session reemplaza el conjunto entero, nunca
// parchea fila por fila (spec §3.3).
func (d *sessionExerciseDao) ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&dbs.SessionExercise{}).Error; err != nil {
			return fmt.Errorf("error clearing session exercises: %w", err)
		}
		for i := range rows {
			rows[i].ID = 0
			rows[i].SessionID = sessionID
			if err := tx.Create(&rows[i]).Error; err != nil {
				return fmt.Errorf("error creating session exercise: %w", err)
			}
		}
		return nil
	})
}
