package constants

import "testing"

func TestIsValidExerciseIntensity(t *testing.T) {
	if !IsValidExerciseIntensity("vigorous") {
		t.Error("vigorous debería ser válido")
	}
	if IsValidExerciseIntensity("extreme") {
		t.Error("extreme no debería ser válido")
	}
}

func TestGetValidExerciseIntensities(t *testing.T) {
	intensities := GetValidExerciseIntensities()
	if len(intensities) != 3 {
		t.Errorf("esperaba 3 intensities, obtuve %d", len(intensities))
	}
}
