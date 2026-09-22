package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// Banner del home de próxima sesión (design.md D6). Shape BREAKING in-place:
// reemplaza al NextSessionResponse anterior (una sola sesión con instancia
// embebida, 204 si no había) — siempre 200 con ambos campos nullable.
type NextSessionResponse struct {
	NextCancelled *NextSessionBannerItem  `json:"next_cancelled"`
	NextTraining  *NextTrainingBannerItem `json:"next_training"`
}

// NextSessionBannerItem es el shape compartido de los banners: la próxima
// sesión del kind correspondiente entre todos los grupos del usuario.
type NextSessionBannerItem struct {
	GroupID     int64   `json:"group_id"`
	GroupName   string  `json:"group_name"`
	Date        string  `json:"date"`
	SessionName *string `json:"session_name"`
}

// NextTrainingBannerItem agrega los datos presenciales cuando el training es
// presencial; un training asincrónico los deja en null.
type NextTrainingBannerItem struct {
	NextSessionBannerItem
	IsPresencial       bool                   `json:"is_presencial"`
	PresencialTimeFrom *string                `json:"presencial_time_from"`
	PresencialTimeTo   *string                `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}
