package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// Errores de negocio del POST seen (design.md D3). El controller los mapea a
// 400 vía errors.Is.
var (
	ErrGamificationInvalidSection = errors.New("section inválida")
	ErrGamificationTeamIDInvalid  = errors.New("team_id solo válido en competitions")
)

// GamificationSeenServiceInterface valida y aplica el escape del punto rojo:
// POST /users/{id}/gamification/seen (self-only, guard en el controller).
type GamificationSeenServiceInterface interface {
	// See borra las filas unseen del usuario para la sección. La whitelist de
	// sections y la regla team_id-solo-competitions viven acá para poder
	// testearse sin HTTP.
	See(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error
}

type gamificationSeenService struct {
	dao daos.GamificationDAOInterface
}

func NewGamificationSeenService(dao daos.GamificationDAOInterface) GamificationSeenServiceInterface {
	return &gamificationSeenService{dao: dao}
}

// validGamificationSection hace la whitelist D3 (valores de
// constants/gamification_section.go).
func validGamificationSection(section string) bool {
	switch section {
	case constants.GamificationSectionStreaks,
		constants.GamificationSectionAchievements,
		constants.GamificationSectionCompetitions:
		return true
	}
	return false
}

// See valida la sección y delega el DELETE: con team_id (solo competitions)
// filtra por equipo; sin team_id limpia TODAS las de la sección, incluidas
// las por-equipo. Cero filas es éxito no-op.
func (s *gamificationSeenService) See(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error {
	if !validGamificationSection(section) {
		return ErrGamificationInvalidSection
	}
	if section != constants.GamificationSectionCompetitions && teamID != nil {
		return ErrGamificationTeamIDInvalid
	}
	if _, err := s.dao.DeleteUnseen(ctx, targetUserID, section, teamID); err != nil {
		customlogger.Error(ctx, "error deleting gamification unseen", err, customlogger.TagMethod("GamificationSeen"))
		return fmt.Errorf("error al marcar novedades vistas")
	}
	return nil
}
