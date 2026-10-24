package dbs

import "time"

// GamificationStreakState cachea el current de cada racha para los triggers
// de notificación de Task 2: el hook post-write de runner session compara el
// current persistido antes y después de la escritura. El GET de rachas NO la
// usa — recalcula fresco sobre el histórico y siempre es exacto.
type GamificationStreakState struct {
	UserID          int64     `gorm:"column:user_id;primaryKey"`
	StrictCurrent   int       `gorm:"column:strict_current;not null;default:0"`
	FlexibleCurrent int       `gorm:"column:flexible_current;not null;default:0"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (GamificationStreakState) TableName() string {
	return "gamification_streak_state"
}
