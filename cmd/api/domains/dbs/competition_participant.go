package dbs

// CompetitionParticipant es la membresía de un corredor a una competencia
// (gamificacion-core D8). PK compuesta; índice propio por user_id para el
// guard del POST /session-instances/:id/runner. Borrado físico junto a la
// competencia (DELETE cancel).
type CompetitionParticipant struct {
	CompetitionID int64 `gorm:"column:competition_id;primaryKey"`
	UserID        int64 `gorm:"column:user_id;primaryKey;index:idx_competition_participant_user"`
}

func (CompetitionParticipant) TableName() string {
	return "competition_participant"
}
