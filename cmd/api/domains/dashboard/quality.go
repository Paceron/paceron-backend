package dashboard

// DataQuality es el bloque que acompaña a toda respuesta de métricas: está
// presente aunque todos los contadores estén en cero, para que el cliente
// distinga "no hay datos" de "los datos están limpios".
//
// OpenSessions queda en 0 cuando se arma con dataQualityFromSeries: la cuenta
// del WHERE que excluye las sesiones wip ya sacó esas filas del result set, así
// que contarlas ahí daría siempre cero y hace falta una segunda consulta.
type DataQuality struct {
	InvalidSeries    int  `json:"invalid_series"`
	DegradedTime     int  `json:"degraded_time"`
	UnknownStatus    int  `json:"unknown_status"`
	OpenSessions     int  `json:"open_sessions"`
	LegacySessions   int  `json:"legacy_sessions"`
	InsufficientData bool `json:"insufficient_data"`
}

// dataQualityFromSeries cuenta los cinco contadores que salen del mismo result
// set de series.
func dataQualityFromSeries(series []Series) DataQuality {
	var quality DataQuality
	for _, s := range series {
		valid := isValidSeries(s.Kind, s)
		if !valid {
			quality.InvalidSeries++
		}
		if timeQuality(s) == TimeQualityDegraded {
			quality.DegradedTime++
		}
		if completionTaxonomy(s) == CompletionUnknown {
			quality.UnknownStatus++
		}
		// legacy_sessions es informativo, no un defecto: cuenta series válidas
		// que entran igual al KPI por una vía sin respaldo completo. Son dos
		// casos que el usuario ve igual, así que van en el mismo contador:
		//   1. sin fila en runner_session (anteriores al registro de sesión)
		//   2. instancia resuelta por nombre, que el backfill no pudo resolver
		if valid && (!s.HasRunnerSession || s.ResolvedByName) {
			quality.LegacySessions++
		}
	}
	quality.InsufficientData = len(validSpeeds(series)) < trendMinSeries
	return quality
}
