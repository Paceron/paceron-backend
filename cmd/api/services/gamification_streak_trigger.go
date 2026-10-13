package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// ErrStreakTrigger es el contrato del trigger de racha para tests: el
// usuario/SW ya vieron el write del PATCH, el fallo acá es best-effort.
var ErrStreakTrigger = fmt.Errorf("error al evaluar rachas")

// streakMilestones son los hitos de racha que generan unseen (design D3):
// cruzado hacia arriba con old < hito y new >= hito.
var streakMilestones = []int{10, 20, 50, 100}

// StreakTriggerServiceInterface es el hook post-write del PATCH de runner
// session hacia finished/interrupted (gamificacion-core D3, tasks 2.2-2.4).
type StreakTriggerServiceInterface interface {
	// OnRunnerSessionCompletion dispara el chequeo en POST-respuesta (sync en
	// el controller, async best-effort en el service). Nil-safe.
	OnRunnerSessionCompletion(ctx *gin.Context, userID int64)
	// EvaluateRunnerCompletion es el core síncrono: compara el current
	// persistido como baseline con el recálculo fresco (lógica del GET),
	// inserta unseen section streaks ante reset o cruce de hito y persiste el
	// estado nuevo.
	EvaluateRunnerCompletion(ctx *gin.Context, userID int64) error
}

type streakTriggerService struct {
	dao     daos.GamificationDAOInterface
	streaks StreakServiceInterface
}

func NewStreakTriggerService(dao daos.GamificationDAOInterface, streaks StreakServiceInterface) StreakTriggerServiceInterface {
	return &streakTriggerService{dao: dao, streaks: streaks}
}

// streakTriggered decide si una racha que pasó de prev a next genera novedad:
// reset (>0 → 0) o cruce de hito hacia arriba. Un salto que pisa varios hitos
// es UNA novedad para esa racha (dedupe del hook: máx una fila por racha,
// hasta 2 con ambas rachas afectadas).
func streakTriggered(prev, next int) bool {
	if prev > 0 && next == 0 {
		return true
	}
	for _, hito := range streakMilestones {
		if prev < hito && next >= hito {
			return true
		}
	}
	return false
}

func (s *streakTriggerService) EvaluateRunnerCompletion(ctx *gin.Context, userID int64) error {
	prev, err := s.dao.GetStreakState(ctx, userID)
	if err != nil {
		return fmt.Errorf("%w: estado previo", ErrStreakTrigger)
	}
	prevStrict, prevFlexible := 0, 0
	if prev != nil {
		prevStrict, prevFlexible = prev.StrictCurrent, prev.FlexibleCurrent
	}

	newStrict, newFlexible, err := s.streaks.ComputeCurrentStreaks(ctx, userID)
	if err != nil {
		return fmt.Errorf("%w: recálculo fresco", ErrStreakTrigger)
	}

	// Una fila por racha afectada. Todas las filas comparten section streaks
	// (unseen no modela la razón, D3): la cantidad solo materializa el hecho.
	var unseen []dbs.GamificationUnseen
	if streakTriggered(prevStrict, newStrict) {
		unseen = append(unseen, dbs.GamificationUnseen{UserID: userID, Section: constants.GamificationSectionStreaks})
	}
	if streakTriggered(prevFlexible, newFlexible) {
		unseen = append(unseen, dbs.GamificationUnseen{UserID: userID, Section: constants.GamificationSectionStreaks})
	}
	if len(unseen) > 0 {
		if err := s.dao.InsertUnseen(ctx, unseen); err != nil {
			return fmt.Errorf("%w: unseen", ErrStreakTrigger)
		}
	}
	// El upsert SIEMPRE persiste el estado fresco, trigger o no: es el
	// baseline previo de la próxima mutación.
	if err := s.dao.UpsertStreakState(ctx, userID, newStrict, newFlexible); err != nil {
		return fmt.Errorf("%w: estado nuevo", ErrStreakTrigger)
	}
	return nil
}

// run es la unidad que el goroutine ejecuta: panic-guard + log y nada más
// (deriva D14: el punto rojo puede llegar tarde o doblarse; el GET exacto
// nunca depende del trigger). Los tests la invocan directo para el camino de
// panic sin depurar del scheduling.
func (s *streakTriggerService) run(hookCtx *gin.Context, userID int64) {
	defer func() {
		if r := recover(); r != nil {
			customlogger.Error(hookCtx, "panic en trigger de racha", fmt.Errorf("%v", r), customlogger.TagMethod("StreakTrigger"))
		}
	}()
	if err := s.EvaluateRunnerCompletion(hookCtx, userID); err != nil {
		customlogger.Error(hookCtx, "trigger de racha falló", err, customlogger.TagMethod("StreakTrigger"))
	}
}

// OnRunnerSessionCompletion encola el chequeo async post-write. El contexto
// de la request muere con la respuesta: el goroutine usa una copia (gin
// Copy) que conserva la ruta para los logs sin heredar la cancelación.
func (s *streakTriggerService) OnRunnerSessionCompletion(ctx *gin.Context, userID int64) {
	if s == nil || s.dao == nil || s.streaks == nil {
		return
	}
	hookCtx := ctx
	if ctx != nil {
		hookCtx = ctx.Copy()
	}
	go func() {
		s.run(hookCtx, userID)
	}()
}
