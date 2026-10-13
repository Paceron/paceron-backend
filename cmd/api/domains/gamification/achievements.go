package gamification

import "time"

// AchievementResponse es una fila del catálogo de logros
// (gamificacion-core design.md D5): shape plano de constants.
type AchievementResponse struct {
	ID          string  `json:"id"`
	Family      string  `json:"family"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	IconKey     string  `json:"icon_key"`
	Threshold   float64 `json:"threshold"`
	Unit        string  `json:"unit"`
}

// AchievementCatalogResponse es la respuesta de GET /achievements/catalog: las
// 21 filas del catálogo, visible para cualquier usuario autenticado.
type AchievementCatalogResponse struct {
	Data []AchievementResponse `json:"data"`
}

// UserAchievementItem es un logro del merge catálogo + progreso de
// GET /users/{id}/achievements. current_level y count llegan en null en v1
// (regla futura); unlocked_at null cuando el logro no está desbloqueado;
// last_unlocked_at replica unlocked_at en v1.
type UserAchievementItem struct {
	AchievementID  string     `json:"achievement_id"`
	CurrentLevel   *int       `json:"current_level"`
	Count          *int       `json:"count"`
	UnlockedAt     *time.Time `json:"unlocked_at"`
	LastUnlockedAt *time.Time `json:"last_unlocked_at"`
}

// UserAchievementsResponse es la respuesta de la vitrina de logros, con
// has_unseen de la sección achievements embebido.
type UserAchievementsResponse struct {
	Data      []UserAchievementItem `json:"data"`
	HasUnseen bool                  `json:"has_unseen"`
}

// StampItem es una estampa de la vitrina (visible en E2 solo como lista, y
// vacía hasta que E4 empiece a emitir estampas de competencia).
type StampItem struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	SourceID   int64     `json:"source_id"`
	SourceName string    `json:"source_name"`
	GrantedAt  time.Time `json:"granted_at"`
	Count      int       `json:"count"`
}

// StampsResponse es la respuesta de GET /users/{id}/stamps.
type StampsResponse struct {
	Data []StampItem `json:"data"`
}
