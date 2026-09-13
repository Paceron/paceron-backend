package constants

import "testing"

func TestIsValidGroupCalendarDayKind(t *testing.T) {
	if !IsValidGroupCalendarDayKind("cancelled") {
		t.Error("cancelled debería ser válido")
	}
	if IsValidGroupCalendarDayKind("flying") {
		t.Error("flying no debería ser válido")
	}
}

func TestGetValidGroupCalendarDayKinds(t *testing.T) {
	kinds := GetValidGroupCalendarDayKinds()
	if len(kinds) != 4 {
		t.Errorf("esperaba 4 kinds, obtuve %d", len(kinds))
	}
}
