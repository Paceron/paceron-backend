package dbs

import "time"

// JoinRequest representa la solicitud de un corredor para unirse a un equipo
// público. Status reusa los mismos 3 valores que constants.InvitationStatus
// (pending/accepted/rejected) — cancelar una solicitud propia borra la fila
// en vez de agregar un 4to estado.
type JoinRequest struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TeamID    int64     `gorm:"column:team_id;not null;index:idx_join_requests_team_id_status,priority:1"`
	RunnerID  int64     `gorm:"column:runner_id;not null;index"`
	Status    string    `gorm:"column:status;not null;default:pending;index:idx_join_requests_team_id_status,priority:2"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (JoinRequest) TableName() string {
	return "join_requests"
}
