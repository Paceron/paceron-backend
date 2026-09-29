package constants

import "testing"

func TestIsValidAttendanceSource(t *testing.T) {
	if !IsValidAttendanceSource("qr") {
		t.Error("'qr' debería ser válido")
	}
	if !IsValidAttendanceSource("manual") {
		t.Error("'manual' debería ser válido")
	}
	if IsValidAttendanceSource("") {
		t.Error("'' no debería ser válido")
	}
	if IsValidAttendanceSource("QR") {
		t.Error("'QR' no debería ser válido: el dominio es case-sensitive")
	}
	if IsValidAttendanceSource("asistido") {
		t.Error("'asistido' está fuera del dominio")
	}
}

func TestGetValidAttendanceSources(t *testing.T) {
	got := GetValidAttendanceSources()
	if len(got) != 2 {
		t.Fatalf("se esperaban 2 valores, se obtuvieron %d: %v", len(got), got)
	}
}
