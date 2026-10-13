package constants

// Secciones de novedades de gamificación (gamificacion-core design.md D3).
// Section de gamification_unseen; el escape de cada punto rojo es POST seen
// para la sección correspondiente.
const (
	GamificationSectionStreaks      = "streaks"
	GamificationSectionAchievements = "achievements"
	GamificationSectionCompetitions = "competitions"
)

// GetValidGamificationSections devuelve las secciones válidas del módulo.
func GetValidGamificationSections() []string {
	return []string{
		GamificationSectionStreaks,
		GamificationSectionAchievements,
		GamificationSectionCompetitions,
	}
}
