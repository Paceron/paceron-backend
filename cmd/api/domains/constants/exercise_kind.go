package constants

type ExerciseKind string

const (
	ExerciseKindWalking    ExerciseKind = "walking"
	ExerciseKindJogging    ExerciseKind = "jogging"
	ExerciseKindElongation ExerciseKind = "elongation"
	ExerciseKindCruising   ExerciseKind = "cruising"
	ExerciseKindRunning    ExerciseKind = "running"
)

func GetValidExerciseKinds() []string {
	return []string{
		string(ExerciseKindWalking),
		string(ExerciseKindJogging),
		string(ExerciseKindElongation),
		string(ExerciseKindCruising),
		string(ExerciseKindRunning),
	}
}

func IsValidExerciseKind(kind string) bool {
	for _, k := range GetValidExerciseKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
