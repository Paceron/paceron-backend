package constants

type SessionExerciseRole string

const (
	SessionExerciseRoleWarmup   SessionExerciseRole = "warmup"
	SessionExerciseRoleMain     SessionExerciseRole = "main"
	SessionExerciseRoleCooldown SessionExerciseRole = "cooldown"
)

func GetValidSessionExerciseRoles() []string {
	return []string{
		string(SessionExerciseRoleWarmup),
		string(SessionExerciseRoleMain),
		string(SessionExerciseRoleCooldown),
	}
}

func IsValidSessionExerciseRole(role string) bool {
	for _, r := range GetValidSessionExerciseRoles() {
		if r == role {
			return true
		}
	}
	return false
}
