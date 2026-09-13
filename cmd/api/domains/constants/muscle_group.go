package constants

type MuscleGroup string

const (
	MuscleGroupCuadriceps     MuscleGroup = "cuadriceps"
	MuscleGroupIsquiotibiales MuscleGroup = "isquiotibiales"
	MuscleGroupGemelos        MuscleGroup = "gemelos"
	MuscleGroupGluteos        MuscleGroup = "gluteos"
	MuscleGroupAductores      MuscleGroup = "aductores"
	MuscleGroupPsoas          MuscleGroup = "psoas"
	MuscleGroupLumbares       MuscleGroup = "lumbares"
	MuscleGroupCore           MuscleGroup = "core"
)

func GetValidMuscleGroups() []string {
	return []string{
		string(MuscleGroupCuadriceps),
		string(MuscleGroupIsquiotibiales),
		string(MuscleGroupGemelos),
		string(MuscleGroupGluteos),
		string(MuscleGroupAductores),
		string(MuscleGroupPsoas),
		string(MuscleGroupLumbares),
		string(MuscleGroupCore),
	}
}

func IsValidMuscleGroup(group string) bool {
	for _, g := range GetValidMuscleGroups() {
		if g == group {
			return true
		}
	}
	return false
}
