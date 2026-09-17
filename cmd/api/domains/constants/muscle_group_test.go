package constants

import "testing"

func TestIsValidMuscleGroup(t *testing.T) {
	if !IsValidMuscleGroup("cuadriceps") {
		t.Error("cuadriceps debería ser válido")
	}
	if IsValidMuscleGroup("triceps") {
		t.Error("triceps no debería ser válido")
	}
}

func TestGetValidMuscleGroups(t *testing.T) {
	groups := GetValidMuscleGroups()
	if len(groups) != 8 {
		t.Errorf("esperaba 8 muscle groups, obtuve %d", len(groups))
	}
}
