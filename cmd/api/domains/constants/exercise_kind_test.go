package constants

import "testing"

func TestIsValidExerciseKind(t *testing.T) {
	if !IsValidExerciseKind("running") {
		t.Error("running debería ser válido")
	}
	if IsValidExerciseKind("flying") {
		t.Error("flying no debería ser válido")
	}
}

func TestGetValidExerciseKinds(t *testing.T) {
	kinds := GetValidExerciseKinds()
	if len(kinds) != 5 {
		t.Errorf("esperaba 5 kinds, obtuve %d", len(kinds))
	}
}
