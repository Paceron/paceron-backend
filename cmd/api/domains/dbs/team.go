package dbs

import "time"

// Team representa un equipo del sistema.
// Cada equipo tiene un owner (debe tener rol "entrenador"), una dirección
// y un conjunto de grupos y usuarios asociados.
type Team struct {
	ID                  int64      `gorm:"column:id;primaryKey"`                                 // ID único del equipo (autoincremental)
	Name                string     `gorm:"column:name;not null"`                                 // Nombre del equipo
	Description         string     `gorm:"column:description"`                                   // Descripción del equipo
	Level               string     `gorm:"column:level"`                                         // Nivel del equipo (ej: "principiante", "avanzado")
	MaxMembers          int64      `gorm:"column:max_members;not null"`                          // Cantidad máxima de integrantes permitidos
	Requirements        string     `gorm:"column:requirements"`                                  // Requerimientos para entrar al equipo
	OwnerID             int64      `gorm:"column:owner_id;not null"`                             // ID del usuario owner (debe tener rol "entrenador")
	MembershipFee       float64    `gorm:"column:membership_fee;not null;default:0"`             // Mensualidad que paga cada corredor al entrenador (0 = gratis)
	Status              string     `gorm:"column:status;not null;default:active"`                // Estado del equipo (active, inactive, archived)
	ShowGroupsToRunners bool       `gorm:"column:show_groups_to_runners;not null;default:false"` // Si los corredores ven a qué grupo pertenece cada compañero
	Country             string     `gorm:"column:country"`                                       // Dirección: país
	Province            string     `gorm:"column:province"`                                      // Dirección: provincia
	City                string     `gorm:"column:city"`                                          // Dirección: ciudad
	Street              string     `gorm:"column:street"`                                        // Dirección: calle
	Number              string     `gorm:"column:number"`                                        // Dirección: número
	DeletedAt           *time.Time `gorm:"column:deleted_at"`                                    // Fecha de eliminación lógica (nil = activo)
	CreatedAt           time.Time  `gorm:"column:created_at;autoCreateTime"`                     // Fecha de creación
	UpdatedAt           time.Time  `gorm:"column:updated_at;autoUpdateTime"`                     // Fecha de última actualización
}

func (Team) TableName() string {
	return "teams"
}
