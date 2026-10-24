package dbs

import "time"

// UserAchievementProgress registra el desbloqueo de un logro del catálogo de
// constants (gamificacion-core design.md D5). La PK compuesta (user_id,
// achievement_id) es además el único mecanismo de idempotencia de la
// evaluación: re-evaluar un logro ya desbloqueado no inserta ni actualiza nada
// (ON CONFLICT DO NOTHING). achievement_id es varchar(64), sin FK al catálogo
// (el catálogo vive en código, no en una tabla).
type UserAchievementProgress struct {
	UserID        int64     `gorm:"column:user_id;primaryKey;index:idx_user_achievement_progress_user"`
	AchievementID string    `gorm:"column:achievement_id;type:varchar(64);primaryKey"`
	UnlockedAt    time.Time `gorm:"column:unlocked_at;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserAchievementProgress) TableName() string {
	return "user_achievement_progress"
}
