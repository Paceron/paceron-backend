package dbs

import "time"

// Invitation representa una invitación de un equipo a un usuario para unirse como corredor.
// El flujo es siempre in-app: el invitado ve sus invitaciones pendientes logueado y decide
// aceptar o rechazar. No hay token secreto ni link mágico en el email.
type Invitation struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	TeamID      int64      `gorm:"column:team_id;not null;index"`
	InviterID   int64      `gorm:"column:inviter_id;not null"`
	InviteeID   int64      `gorm:"column:invitee_id;not null;index"`
	GroupID     *int64     `gorm:"column:group_id"` // Grupo elegido al invitar (nil = grupo principal del equipo al aceptar)
	Status      string     `gorm:"column:status;not null;default:pending"`
	ExpiresAt   time.Time  `gorm:"column:expires_at;not null"`
	RespondedAt *time.Time `gorm:"column:responded_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Invitation) TableName() string {
	return "invitations"
}
