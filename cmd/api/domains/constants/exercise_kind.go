package constants

type ExerciseKind string

// Kinds originales. Se mantienen válidos a propósito: el frontend los
// reemplazó por los de abajo (2026-10-12), pero las filas que no.passaron por
// la migración de datos —o que llegue de un cliente viejo— tienen que seguir
// aceptándose en lugar de empezar a fallar con 400.
//
// No existe un enum de Postgres acá: `exercises.kind` es una columna text y la
// validación es esta función. Por eso agregar valores es solo agregar
// constantes, sin migración de schema.
const (
	ExerciseKindWalking    ExerciseKind = "walking"
	ExerciseKindJogging    ExerciseKind = "jogging"
	ExerciseKindElongation ExerciseKind = "elongation"
	ExerciseKindCruising   ExerciseKind = "cruising"
	ExerciseKindRunning    ExerciseKind = "running"
)

// Kinds vigentes. `elongacion` es un rename de `elongation` (se le saca la tilde
// final por convención de snake_case), y los otros cuatro reorganizan el
// criterio: el tipo ahora declara qué datos lo definen, no solo "cómo se
// corrió".
const (
	ExerciseKindLiviano            ExerciseKind = "liviano"
	ExerciseKindElongacion         ExerciseKind = "elongacion"
	ExerciseKindRitmoContinuo      ExerciseKind = "ritmo_continuo"
	ExerciseKindVelocidadTiempo    ExerciseKind = "velocidad_tiempo"
	ExerciseKindVelocidadDistancia ExerciseKind = "velocidad_distancia"
)

func GetValidExerciseKinds() []string {
	return []string{
		string(ExerciseKindLiviano),
		string(ExerciseKindElongacion),
		string(ExerciseKindRitmoContinuo),
		string(ExerciseKindVelocidadTiempo),
		string(ExerciseKindVelocidadDistancia),
		// Los originales siguen en la lista: ver la nota de los const de arriba.
		string(ExerciseKindWalking),
		string(ExerciseKindJogging),
		string(ExerciseKindElongation),
		string(ExerciseKindCruising),
		string(ExerciseKindRunning),
	}
}

// Mapeo de los kinds originales a los vigentes, para la migración de datos.
// Vive acá y no en la migración SQL para que los tests puedan verificar que
// cubre los 10 valores válidos — un kind viejo que se olvide en el UPDATE
// quedaría con datos que el frontend nuevo no reconoce.
//
// Nota: `jogging` y `cruising` colapsan a `ritmo_continuo`. Es intencional
// (el trote es ritmo sostenido), pero implica que ejercicios migrados a ese
// kind pueden no cumplir la matriz nueva, que exige distancia Y tiempo.
var legacyExerciseKindMigrations = map[ExerciseKind]ExerciseKind{
	ExerciseKindWalking:    ExerciseKindLiviano,
	ExerciseKindJogging:    ExerciseKindRitmoContinuo,
	ExerciseKindElongation: ExerciseKindElongacion,
	ExerciseKindCruising:   ExerciseKindRitmoContinuo,
	ExerciseKindRunning:    ExerciseKindVelocidadDistancia,
}

// LegacyExerciseKindMigrations expone el mapeo original -> vigente para que la
// migración de datos (infrastructure/postgresdb) arme el UPDATE desde acá en vez
// de escribir los valores a mano en SQL, que se desincronizarían de estas
// constantes sin ningún aviso.
func LegacyExerciseKindMigrations() map[ExerciseKind]ExerciseKind {
	out := make(map[ExerciseKind]ExerciseKind, len(legacyExerciseKindMigrations))
	for k, v := range legacyExerciseKindMigrations {
		out[k] = v
	}
	return out
}

// Devuelve el kind vigente al que corresponde un kind original, y el original
// si no hay mapeo (los vigentes pasan sin cambios).
func MigrateExerciseKind(kind ExerciseKind) ExerciseKind {
	if mapped, ok := legacyExerciseKindMigrations[kind]; ok {
		return mapped
	}
	return kind
}

func IsValidExerciseKind(kind string) bool {
	for _, k := range GetValidExerciseKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
