package dbs

// CompetitionWinner marca los ganadores al cierre (gamificacion-core D8). El
// empate exacto "todos ganan" inserta N filas con la misma competition_id.
// Índice propio por user_id. Las filas se generan en el close (Task 6).
type CompetitionWinner struct {
	CompetitionID int64 `gorm:"column:competition_id;primaryKey"`
	UserID        int64 `gorm:"column:user_id;primaryKey;index:idx_competition_winner_user"`
}

func (CompetitionWinner) TableName() string {
	return "competition_winner"
}
