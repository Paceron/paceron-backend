package dbs

import "time"

// GamificationUnseen guarda el hecho "hay novedades de gamificación que ver"
// (gamificacion-core design.md D3): el punto rojo del front es derivable de la
// existencia de filas, no hay contador ni estado por ítem. Section es
// streaks/achievements/competitions; TeamID solo aplica a competitions. El
// escape es POST seen (borra filas de la sección) — hasta Task 2 no hay
// escritores.
type GamificationUnseen struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_gamification_unseen_user_section,priority:1"`
	Section   string    `gorm:"column:section;type:varchar(32);not null;index:idx_gamification_unseen_user_section,priority:2"`
	TeamID    *int64    `gorm:"column:team_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (GamificationUnseen) TableName() string {
	return "gamification_unseen"
}
