package dbs

import "time"

// GamificationStamp es una estampa de la vitrina (gamificacion-core design.md
// D5). En v1 SOLO recibe emisiones de type=competition (E4): la tabla se crea
// vacía y el GET de estampas responde lista vacía hasta entonces.
type GamificationStamp struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	UserID     int64     `gorm:"column:user_id;not null;index:idx_gamification_stamp_user"`
	Type       string    `gorm:"column:type;type:varchar(32);not null"`
	SourceID   int64     `gorm:"column:source_id;not null"`
	SourceName string    `gorm:"column:source_name"`
	GrantedAt  time.Time `gorm:"column:granted_at;not null"`
	Count      int       `gorm:"column:count;not null;default:1"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (GamificationStamp) TableName() string {
	return "gamification_stamp"
}
