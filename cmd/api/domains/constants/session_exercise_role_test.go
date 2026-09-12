package constants

import "testing"

func TestIsValidSessionExerciseRole(t *testing.T) {
	if !IsValidSessionExerciseRole("warmup") {
		t.Error("warmup debería ser válido")
	}
	if IsValidSessionExerciseRole("stretch") {
		t.Error("stretch no debería ser válido")
	}
}

func TestGetValidSessionExerciseRoles(t *testing.T) {
	roles := GetValidSessionExerciseRoles()
	if len(roles) != 3 {
		t.Errorf("esperaba 3 roles, obtuve %d", len(roles))
	}
}
