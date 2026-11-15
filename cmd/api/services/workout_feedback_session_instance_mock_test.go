package services

import (
	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
)

// ExerciseAndTeamForSessionInstance emula la lectura combinada del DAO: sin fn
// explícita delega en los campos single, conservando el orden de validación
// viejo (pertenencia primero; si no pertenece, no se deriva team).
func (m *mockSessionInstanceDao) ExerciseAndTeamForSessionInstance(ctx *gin.Context, instanceID, exerciseInstanceID int64) (bool, *int64, error) {
	belongs, err := m.ExerciseBelongsToSessionInstance(ctx, instanceID, exerciseInstanceID)
	if err != nil || !belongs {
		return belongs, nil, err
	}
	teamID, err := m.TeamIDForSessionInstance(ctx, instanceID)
	if err != nil {
		return false, nil, err
	}
	return true, teamID, nil
}

// feedbackInstanceDao inyecta y cuenta la lectura combinada sin tocar los
// campos del mock compartido.
type feedbackInstanceDao struct {
	*mockSessionInstanceDao
	exerciseTeamFn func(ctx *gin.Context, instanceID, exerciseInstanceID int64) (bool, *int64, error)
}

func (m *feedbackInstanceDao) ExerciseAndTeamForSessionInstance(ctx *gin.Context, instanceID, exerciseInstanceID int64) (bool, *int64, error) {
	if m.exerciseTeamFn != nil {
		return m.exerciseTeamFn(ctx, instanceID, exerciseInstanceID)
	}
	return m.mockSessionInstanceDao.ExerciseAndTeamForSessionInstance(ctx, instanceID, exerciseInstanceID)
}

var _ daos.SessionInstanceDaoInterface = (*feedbackInstanceDao)(nil)
