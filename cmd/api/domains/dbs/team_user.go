package dbs

import "time"

// TeamUser representa la asociación de un usuario a un equipo.
// Cada registro indica que un usuario pertenece a un equipo con un rol específico
// (entrenador, corredor) y un estado de la asociación.
type TeamUser struct {
	ID              int64      `gorm:"column:id;primaryKey"`                    // ID único de la asociación
	TeamID          int64      `gorm:"column:team_id;not null"`                 // ID del equipo
	UserID          int64      `gorm:"column:user_id;not null"`                 // ID del usuario
	RoleInTeam      string     `gorm:"column:role_in_team;not null"`            // Rol del usuario en el equipo (entrenador, corredor)
	Status          string     `gorm:"column:status;not null;default:active"`   // Estado de la asociación (active, inactive)
	AssignmentDate  time.Time  `gorm:"column:assignment_date;not null"`         // Fecha en que se asoció el usuario al equipo
	DeletedAt       *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (TeamUser) TableName() string {
	return "team_users"
}
