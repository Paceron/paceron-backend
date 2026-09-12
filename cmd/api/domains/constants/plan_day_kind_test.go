package constants

import "testing"

func TestIsValidPlanDayKind(t *testing.T) {
	if !IsValidPlanDayKind("training") {
		t.Error("training debería ser válido")
	}
	if IsValidPlanDayKind("recovery") {
		t.Error("recovery no debería ser válido")
	}
}

func TestGetValidPlanDayKinds(t *testing.T) {
	kinds := GetValidPlanDayKinds()
	if len(kinds) != 3 {
		t.Errorf("esperaba 3 kinds, obtuve %d", len(kinds))
	}
}
