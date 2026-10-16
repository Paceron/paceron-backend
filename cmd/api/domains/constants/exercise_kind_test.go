package constants

import "testing"

func TestIsValidExerciseKind(t *testing.T) {
	tests := []struct {
		kind  string
		valid bool
		why   string
	}{
		// Vigentes
		{"liviano", true, "liviano es un tipo vigente"},
		{"elongacion", true, "elongacion es un tipo vigente"},
		{"ritmo_continuo", true, "ritmo_continuo es un tipo vigente"},
		{"velocidad_tiempo", true, "velocidad_tiempo es un tipo vigente"},
		{"velocidad_distancia", true, "velocidad_distancia es un tipo vigente"},

		// Originales: siguen siendo válidos a propósito, para que una fila sin
		// migrar o un cliente viejo no empiece a recibir 400.
		{"walking", true, "walking debe seguir siendo válido (retrocompatibilidad)"},
		{"jogging", true, "jogging debe seguir siendo válido (retrocompatibilidad)"},
		{"elongation", true, "elongation debe seguir siendo válido (retrocompatibilidad)"},
		{"cruising", true, "cruising debe seguir siendo válido (retrocompatibilidad)"},
		{"running", true, "running debe seguir siendo válido (retrocompatibilidad)"},

		// Inválidos
		{"flying", false, "flying no es un tipo de ejercicio"},
		{"", false, "el kind vacío no es válido"},
		{"LIVIANO", false, "el kind es case-sensitive"},
		{"velocidad", false, "un prefijo sin sufijo no es válido"},
		{" elongacion", false, "el kind no admite espacios al inicio"},
	}
	for _, tt := range tests {
		if got := IsValidExerciseKind(tt.kind); got != tt.valid {
			t.Errorf("IsValidExerciseKind(%q) = %v, esperaba %v (%s)", tt.kind, got, tt.valid, tt.why)
		}
	}
}

func TestGetValidExerciseKinds(t *testing.T) {
	kinds := GetValidExerciseKinds()
	if len(kinds) != 10 {
		t.Errorf("esperaba 10 kinds (5 vigentes + 5 originales), obtuve %d: %v", len(kinds), kinds)
	}

	seen := map[string]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("kind duplicado en GetValidExerciseKinds: %q", k)
		}
		seen[k] = true
		if !IsValidExerciseKind(k) {
			t.Errorf("IsValidExerciseKind(%q) = false pero está en GetValidExerciseKinds", k)
		}
	}
}

func TestGetValidExerciseKinds_CubreTodosLosValores(t *testing.T) {
	// Si se agrega una constante nueva sin sumarla a GetValidExerciseKinds, el
	// servicio la rechaza con 400 y el frontend no puede crear el ejercicio.
	want := []string{
		"liviano", "elongacion", "ritmo_continuo", "velocidad_tiempo", "velocidad_distancia",
		"walking", "jogging", "elongation", "cruising", "running",
	}
	got := GetValidExerciseKinds()
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("falta %q en GetValidExerciseKinds()", w)
		}
	}
}

func TestMigrateExerciseKind(t *testing.T) {
	tests := []struct {
		in, want string
		why      string
	}{
		{"walking", "liviano", "caminata pasa a liviano"},
		{"jogging", "ritmo_continuo", "trote pasa a ritmo continuo"},
		{"elongation", "elongacion", "elongación solo pierde la tilde final"},
		{"cruising", "ritmo_continuo", "ritmo continuo mantiene su nombre"},
		{"running", "velocidad_distancia", "corrida pasa a velocidad en distancia"},

		// Los vigentes pasan sin cambios — migrar una tabla ya migrada es no-op.
		{"liviano", "liviano", "un kind vigente no cambia"},
		{"elongacion", "elongacion", "un kind vigente no cambia"},
		{"ritmo_continuo", "ritmo_continuo", "un kind vigente no cambia"},
		{"velocidad_tiempo", "velocidad_tiempo", "un kind vigente no cambia"},
		{"velocidad_distancia", "velocidad_distancia", "un kind vigente no cambia"},

		// Desconocido pasa intacto: la migración no debe inventar.
		{"flying", "flying", "un kind desconocido no se toca"},
		{"", "", "el kind vacío no se toca"},
	}
	for _, tt := range tests {
		if got := string(MigrateExerciseKind(ExerciseKind(tt.in))); got != tt.want {
			t.Errorf("MigrateExerciseKind(%q) = %q, esperaba %q (%s)", tt.in, got, tt.want, tt.why)
		}
	}
}

func TestMigrateExerciseKind_Inmutable(t *testing.T) {
	// Un kind que viene de un dato ya vigente tiene que devolver un valor
	// VÁLIDO: si la migración produjera algo que IsValidExerciseKind rechaza,
	// el UPDATE escribiría datos que la API ya no acepta.
	for _, k := range GetValidExerciseKinds() {
		migrated := MigrateExerciseKind(ExerciseKind(k))
		if !IsValidExerciseKind(string(migrated)) {
			t.Errorf("MigrateExerciseKind(%q) = %q, que IsValidExerciseKind rechaza", k, migrated)
		}
	}
}

func TestMigrateExerciseKind_EsIdempotente(t *testing.T) {
	// Correr la migración dos veces no puede seguir moviendo el dato: si
	// MigrateExerciseKind no fuera idempotente, un UPDATE re-ejecutado por
	//error dejaría filas en un estado distinto al esperado.
	for _, k := range []string{"walking", "jogging", "elongation", "cruising", "running"} {
		once := MigrateExerciseKind(ExerciseKind(k))
		twice := MigrateExerciseKind(once)
		if once != twice {
			t.Errorf("migrar %q dos veces cambió el valor: %q → %q → %q", k, k, once, twice)
		}
	}
}

func TestMigrateExerciseKind_CubreTodosLosOriginales(t *testing.T) {
	// Si se agrega un kind original sin mapear, sus filas quedarían con un
	// valor que el frontend nuevo no reconoce.
	originales := []string{"walking", "jogging", "elongation", "cruising", "running"}
	for _, k := range originales {
		migrated := string(MigrateExerciseKind(ExerciseKind(k)))
		if migrated == k {
			t.Errorf("el kind original %q no tiene mapeo: quedaría sin migrar", k)
		}
		if !IsValidExerciseKind(migrated) {
			t.Errorf("el kind original %q mapea a %q, que no es válido", k, migrated)
		}
	}
}

func TestIsDashboardMetricKind(t *testing.T) {
	tests := []struct {
		kind  string
		valid bool
		why   string
	}{
		{"ritmo_continuo", true, "tiene distancia y tiempo"},
		{"velocidad_tiempo", true, "tiene tiempo"},
		{"velocidad_distancia", true, "tiene distancia"},

		// Válidos del modelo, pero NO filtrables: son bloques warmup/cooldown, y el
		// filtro role = 'main' del dashboard deja sus series todas afuera.
		{"liviano", false, "el dashboard no puede medir un calentamiento"},
		{"elongacion", false, "el dashboard no puede medir una elongación"},

		// Legacy e inventados.
		{"running", false, "los legacy no se miden"},
		{"volando", false, "un tipo que no existe"},
		{"", false, "el kind vacío no se filtra"},
	}
	for _, tt := range tests {
		if got := IsDashboardMetricKind(tt.kind); got != tt.valid {
			t.Errorf("IsDashboardMetricKind(%q) = %v, esperaba %v (%s)", tt.kind, got, tt.valid, tt.why)
		}
	}
}

// El filtro de tipo del dashboard y la lista de tipos que el front muestra son listas
// DISTINTAS y no puede pasar a ser la misma: /filters.exercise_types ofrece los cinco
// tipos vigentes, pero el filtro solo acepta los tres con métrica.
func TestDashboardMetricKindsEsSubconjuntoDeLosVigentes(t *testing.T) {
	if len(DashboardMetricKinds()) != 3 {
		t.Errorf("esperaba 3 tipos con métrica, obtuve %d: %v", len(DashboardMetricKinds()), DashboardMetricKinds())
	}
	seen := map[string]bool{}
	for _, kind := range DashboardMetricKinds() {
		if seen[kind] {
			t.Errorf("tipo duplicado en DashboardMetricKinds: %q", kind)
		}
		seen[kind] = true
		if !IsCurrentExerciseKind(kind) {
			t.Errorf("%q tiene métrica pero no es un tipo vigente", kind)
		}
		if !IsValidExerciseKind(kind) {
			t.Errorf("%q tiene métrica pero no es un tipo válido para escribir", kind)
		}
	}
	for _, kind := range CurrentExerciseKinds() {
		if IsDashboardMetricKind(kind) {
			continue
		}
		if !IsValidExerciseKind(kind) {
			t.Errorf("%q quedó fuera de la métrica y además no es un tipo válido", kind)
		}
	}
}
