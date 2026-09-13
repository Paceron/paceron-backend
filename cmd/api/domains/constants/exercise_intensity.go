package constants

type ExerciseIntensity string

const (
	ExerciseIntensityLight    ExerciseIntensity = "light"
	ExerciseIntensityModerate ExerciseIntensity = "moderate"
	ExerciseIntensityVigorous ExerciseIntensity = "vigorous"
)

func GetValidExerciseIntensities() []string {
	return []string{
		string(ExerciseIntensityLight),
		string(ExerciseIntensityModerate),
		string(ExerciseIntensityVigorous),
	}
}

func IsValidExerciseIntensity(intensity string) bool {
	for _, i := range GetValidExerciseIntensities() {
		if i == intensity {
			return true
		}
	}
	return false
}
