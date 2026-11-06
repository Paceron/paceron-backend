package dashboard

// TrainerHomeKpisResponse es el resumen operativo y financiero del Home del
// entrenador. Los campos de sesiones y cobranzas responden al período; la
// estructura corresponde al estado actual del entrenador.
type TrainerHomeKpisResponse struct {
	Period         string                 `json:"period"`
	ReferenceDate  string                 `json:"reference_date"`
	From           string                 `json:"from"`
	To             string                 `json:"to"`
	DaysConsidered int                    `json:"days_considered"`
	Sessions       TrainerHomeSessions    `json:"sessions"`
	Structure      TrainerHomeStructure   `json:"structure"`
	Collections    TrainerHomeCollections `json:"collections"`
}

type TrainerHomeSessions struct {
	Scheduled           int64 `json:"scheduled"`
	Realized            int64 `json:"realized"`
	PresencialScheduled int64 `json:"presencial_scheduled"`
	PresencialRealized  int64 `json:"presencial_realized"`
}

type TrainerHomeStructure struct {
	ActiveTeams   int64 `json:"active_teams"`
	ActiveRunners int64 `json:"active_runners"`
}

type TrainerHomeCollections struct {
	CurrencyID    string  `json:"currency_id"`
	NetCollected  float64 `json:"net_collected"`
	PendingAmount float64 `json:"pending_amount"`
	PendingCount  int     `json:"pending_count"`
}
