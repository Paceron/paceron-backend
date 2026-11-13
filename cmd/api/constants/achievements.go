package constants

// Catálogo de logros (gamificacion-core D5). Lista VERSIONADA en constants Go,
// NO en una tabla: cada fila es un logro "once" de un solo threshold y la
// familia es metadata de agrupación del front (la escalera multi-threshold y
// los counters son regla futura, fuera de v1). Agregar/mutar filas requiere
// subir AchievementCatalogVersion.
const AchievementCatalogVersion = "v2"

const (
	AchievementFamilyConsistency = "consistency"
	AchievementFamilyVolume      = "volume"
	AchievementFamilySessions    = "sessions"
	AchievementFamilyGPS         = "gps"
	AchievementFamilyEarlyBird   = "early_bird"
	AchievementFamilyTeam        = "team"
)

const (
	AchievementUnitSessions = "sessions"
	AchievementUnitDays     = "days"
	AchievementUnitKm       = "km"
)

// AchievementRow es una fila del catálogo de logros.
type AchievementRow struct {
	ID          string
	Family      string
	Name        string
	Description string
	IconKey     string
	IconName    string
	Threshold   float64
	Unit        string
}

// AchievementCatalog lista el catálogo lógico de v1 (21 filas: consistency 6
// + volume 4 + sessions 5 + gps 3 + early_bird 1 + team 2). v2 versiona la
// metadata completa de cada fila, incluido IconName; los 21 IDs, thresholds y
// reglas de desbloqueo siguen siendo el set lógico de v1. Los extras
// (early_10, distance_250k/500k, attendance_20) quedan FUERA de ese set a propósito.
var AchievementCatalog = []AchievementRow{
	{ID: "first_session", Family: AchievementFamilyConsistency, Name: "Primer paso", Description: "Terminaste tu primera sesión", IconKey: "first_session", IconName: "run", Threshold: 1, Unit: AchievementUnitSessions},
	{ID: "streak_3", Family: AchievementFamilyConsistency, Name: "Racha de 3", Description: "3 días seguidos con entrenamiento completo", IconKey: "streak_3", IconName: "fire", Threshold: 3, Unit: AchievementUnitDays},
	{ID: "streak_7", Family: AchievementFamilyConsistency, Name: "Racha de 7", Description: "7 días seguidos con entrenamiento completo", IconKey: "streak_7", IconName: "fire", Threshold: 7, Unit: AchievementUnitDays},
	{ID: "streak_14", Family: AchievementFamilyConsistency, Name: "Racha de 14", Description: "14 días seguidos con entrenamiento completo", IconKey: "streak_14", IconName: "fire", Threshold: 14, Unit: AchievementUnitDays},
	{ID: "streak_30", Family: AchievementFamilyConsistency, Name: "Racha de 30", Description: "30 días seguidos con entrenamiento completo", IconKey: "streak_30", IconName: "fire", Threshold: 30, Unit: AchievementUnitDays},
	{ID: "streak_60", Family: AchievementFamilyConsistency, Name: "Racha de 60", Description: "60 días seguidos con entrenamiento completo", IconKey: "streak_60", IconName: "fire", Threshold: 60, Unit: AchievementUnitDays},

	{ID: "distance_10k", Family: AchievementFamilyVolume, Name: "Primeros 10K", Description: "Sumaste 10 km de distancia en tu historial", IconKey: "distance_10k", IconName: "map-marker-distance", Threshold: 10, Unit: AchievementUnitKm},
	{ID: "distance_50k", Family: AchievementFamilyVolume, Name: "50K acumulados", Description: "Sumaste 50 km de distancia en tu historial", IconKey: "distance_50k", IconName: "map-marker-distance", Threshold: 50, Unit: AchievementUnitKm},
	{ID: "distance_100k", Family: AchievementFamilyVolume, Name: "100K acumulados", Description: "Sumaste 100 km de distancia en tu historial", IconKey: "distance_100k", IconName: "map-marker-distance", Threshold: 100, Unit: AchievementUnitKm},
	{ID: "distance_42k_single", Family: AchievementFamilyVolume, Name: "42K en una sesión", Description: "Una sola sesión acumuló al menos 42.2 km", IconKey: "distance_42k_single", IconName: "run-fast", Threshold: 42.2, Unit: AchievementUnitKm},

	{ID: "sessions_5", Family: AchievementFamilySessions, Name: "5 sesiones", Description: "Terminaste 5 sesiones en total", IconKey: "sessions_5", IconName: "calendar-check", Threshold: 5, Unit: AchievementUnitSessions},
	{ID: "sessions_25", Family: AchievementFamilySessions, Name: "25 sesiones", Description: "Terminaste 25 sesiones en total", IconKey: "sessions_25", IconName: "calendar-check", Threshold: 25, Unit: AchievementUnitSessions},
	{ID: "sessions_50", Family: AchievementFamilySessions, Name: "50 sesiones", Description: "Terminaste 50 sesiones en total", IconKey: "sessions_50", IconName: "calendar-check", Threshold: 50, Unit: AchievementUnitSessions},
	{ID: "sessions_100", Family: AchievementFamilySessions, Name: "100 sesiones", Description: "Terminaste 100 sesiones en total", IconKey: "sessions_100", IconName: "calendar-check", Threshold: 100, Unit: AchievementUnitSessions},
	{ID: "sessions_200", Family: AchievementFamilySessions, Name: "200 sesiones", Description: "Terminaste 200 sesiones en total", IconKey: "sessions_200", IconName: "calendar-check", Threshold: 200, Unit: AchievementUnitSessions},

	{ID: "gps_first", Family: AchievementFamilyGPS, Name: "Primer GPS", Description: "Terminaste una sesión con recorrido GPS", IconKey: "gps_first", IconName: "map-marker-path", Threshold: 1, Unit: AchievementUnitSessions},
	{ID: "gps_10", Family: AchievementFamilyGPS, Name: "10 GPS", Description: "Terminaste 10 sesiones con recorrido GPS", IconKey: "gps_10", IconName: "map-marker-path", Threshold: 10, Unit: AchievementUnitSessions},
	{ID: "gps_50", Family: AchievementFamilyGPS, Name: "50 GPS", Description: "Terminaste 50 sesiones con recorrido GPS", IconKey: "gps_50", IconName: "map-marker-path", Threshold: 50, Unit: AchievementUnitSessions},

	{ID: "early_5", Family: AchievementFamilyEarlyBird, Name: "Madrugador", Description: "5 sesiones empezadas antes de las 8:00", IconKey: "early_5", IconName: "weather-sunset-up", Threshold: 5, Unit: AchievementUnitSessions},

	{ID: "team_first", Family: AchievementFamilyTeam, Name: "Primer día en equipo", Description: "Tu primera asistencia confirmada", IconKey: "team_first", IconName: "account-group", Threshold: 1, Unit: AchievementUnitDays},
	{ID: "team_10", Family: AchievementFamilyTeam, Name: "10 días en equipo", Description: "10 días distintos con asistencia confirmada", IconKey: "team_10", IconName: "account-multiple-check", Threshold: 10, Unit: AchievementUnitDays},
}

// AchievementUnits válidos del catálogo (pin del test de shape).
func AchievementUnits() []string {
	return []string{AchievementUnitSessions, AchievementUnitDays, AchievementUnitKm}
}
