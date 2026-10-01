package constants

// AttendanceSource describe cómo se cargó una asistencia. Es un dominio cerrado:
// la columna `attendances.source` es NOT NULL y la grilla la muestra como
// procedencia, así que un valor fuera de este conjunto sería un defecto de datos
// visible para el usuario.
type AttendanceSource string

const (
	// AttendanceSourceQR es la asistencia que registró el propio corredor al
	// escanear el QR. Es la única procedencia posible de toda fila anterior al
	// change de gestión de asistencia (no existía el alta manual), razón por la
	// cual el backfill de la migración usa 'qr' y no 'manual'.
	AttendanceSourceQR AttendanceSource = "qr"
	// AttendanceSourceManual es la asistencia que cargó el entrenador desde el
	// panel, vía POST /attendance/bulk.
	AttendanceSourceManual AttendanceSource = "manual"
)

// GetValidAttendanceSources devuelve el dominio completo de `source`, para
// validación y para tests.
func GetValidAttendanceSources() []string {
	return []string{
		string(AttendanceSourceQR),
		string(AttendanceSourceManual),
	}
}

// IsValidAttendanceSource indica si un valor pertenece al dominio de `source`.
func IsValidAttendanceSource(source string) bool {
	for _, s := range GetValidAttendanceSources() {
		if s == source {
			return true
		}
	}
	return false
}
