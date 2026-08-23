package dbs

import "time"

// PushToken representa el token de push notifications de un dispositivo. Upsert
// por Token (uniqueIndex), no por UserID: si el mismo dispositivo pasa a otra
// cuenta logueada, el siguiente registro reescribe el dueño solo, sin necesitar
// un endpoint de "desvincular" en logout.
type PushToken struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	UserID    int64     `gorm:"column:user_id;not null;index"`
	Token     string    `gorm:"column:token;not null;uniqueIndex"`
	Platform  string    `gorm:"column:platform;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (PushToken) TableName() string {
	return "push_tokens"
}
