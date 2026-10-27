package dashboard

import "simple-arq-golang/cmd/api/domains/constants"

// Alias de los kinds para las tablas de test: las constantes de
// domains/constants son de tipo ExerciseKind y las funciones del paquete
// toman string.
const (
	kindLiviano            = string(constants.ExerciseKindLiviano)
	kindElongacion         = string(constants.ExerciseKindElongacion)
	kindRitmoContinuo      = string(constants.ExerciseKindRitmoContinuo)
	kindVelocidadTiempo    = string(constants.ExerciseKindVelocidadTiempo)
	kindVelocidadDistancia = string(constants.ExerciseKindVelocidadDistancia)
	kindLegacyWalking      = string(constants.ExerciseKindWalking)
	kindLegacyJogging      = string(constants.ExerciseKindJogging)
	kindLegacyElongation   = string(constants.ExerciseKindElongation)
	kindLegacyCruising     = string(constants.ExerciseKindCruising)
	kindLegacyRunning      = string(constants.ExerciseKindRunning)
)

func ptrInt(v int) *int           { return &v }
func ptrInt64(v int64) *int64     { return &v }
func ptrFloat(v float64) *float64 { return &v }
func ptrString(v string) *string  { return &v }

// achievedFor arma lo logrado sin pasar por achievedFromSeries, para que los
// tests de cumplimiento y banda fijen a mano las magnitudes que comparan.
func achievedFor(timeMs int64, distanceM *float64) SeriesAchieved {
	return SeriesAchieved{TimeMs: timeMs, DistanceM: distanceM}
}

func evaluable(kind string, minutes, distanceM *int, timeMs int64, achievedDistanceM *float64) Evaluable {
	return Evaluable{
		Kind:     kind,
		Targets:  targetFromExerciseInstance(minutes, distanceM),
		Achieved: achievedFor(timeMs, achievedDistanceM),
	}
}
