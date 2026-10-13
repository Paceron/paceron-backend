package dashboard

import (
	"strings"
	"unicode"
)

// accentFolds es el plegado de acentos del español (y de la ü). Se hace con una
// tabla explícita y no con unicode/norm: el catálogo es en español y una lista
// cerrada y testeable no depende de las tablas de Unicode que trae el toolchain.
var accentFolds = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c',
}

// NormalizeExerciseName baja el nombre de un ejercicio a una forma comparable:
// minúsculas, sin acentos, sin puntuación y con espacios colapsados.
//
// Existe por el fallback por nombre de /exercises/{id}/series: las
// ExerciseInstance anteriores al change de instancia-referencia-catálogo no
// tienen source_exercise_id, así que la única forma de saber a qué ejercicio de
// catálogo pertenecían es comparar el nombre copiado contra el catálogo del
// mismo dueño. Sin normalizar, "Distancia 4 km" y "distancia 4 km." serían dos
// ejercicios distintos y el histórico se perdería en silencio.
func NormalizeExerciseName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	spacePending := false
	for _, r := range strings.ToLower(name) {
		if folded, ok := accentFolds[r]; ok {
			r = folded
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			// Todo lo que no sea letra o dígito separa palabras, puntuación
			// incluida: "ritmo-continuo", "Ritmo — Continuo" y "ritmo / continuo"
			// son el mismo ejercicio para quien lo escribió. Los separadores
			// repetidos no acumulan espacios.
			spacePending = b.Len() > 0
			continue
		}
		if spacePending {
			b.WriteRune(' ')
			spacePending = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// CatalogExercise es la proyección del catálogo que el fallback por nombre
// necesita: id, dueño y nombre.
type CatalogExercise struct {
	ID      int64
	OwnerID int64
	Name    string
}

// nameKey acota la coincidencia al mismo dueño. `exercises` es catálogo por
// dueño, así que dos entrenadores distintos pueden tener "Distancia 4 km" con
// objetivos distintos: sin acotar por owner_id el dashboard mostraría el objetivo
// de un entrenador evaluando las sesiones de otro.
type nameKey struct {
	ownerID int64
	name    string
}

// NameIndex resuelve nombres de ExerciseInstance contra el catálogo vigente.
type NameIndex struct {
	byName    map[nameKey]int64
	ambiguous map[nameKey]bool
}

// NewNameIndex arma el índice desde el catálogo completo de los dueños que
// interesan. Un nombre normalizado que matchea más de un ejercicio del mismo
// dueño resuelve al `exercises.id` más bajo y queda marcado como ambiguo: elegir
// por otro criterio haría que el fallback dependiera del orden de una query.
func NewNameIndex(catalog []CatalogExercise) *NameIndex {
	index := &NameIndex{
		byName:    make(map[nameKey]int64, len(catalog)),
		ambiguous: map[nameKey]bool{},
	}
	for _, exercise := range catalog {
		key := nameKey{ownerID: exercise.OwnerID, name: NormalizeExerciseName(exercise.Name)}
		if key.name == "" {
			continue
		}
		current, seen := index.byName[key]
		if !seen {
			index.byName[key] = exercise.ID
			continue
		}
		index.ambiguous[key] = true
		if exercise.ID < current {
			index.byName[key] = exercise.ID
		}
	}
	return index
}

// Resolve devuelve el ejercicio de catálogo al que pertenece una instancia sin
// source_exercise_id. ambiguous es true cuando el nombre matcheó más de un
// ejercicio del mismo dueño: esos casos se cuentan aparte para que el fallback
// sea visible en vez de silencioso.
func (i *NameIndex) Resolve(ownerID int64, instanceName string) (catalogID int64, ambiguous bool, ok bool) {
	key := nameKey{ownerID: ownerID, name: NormalizeExerciseName(instanceName)}
	if key.name == "" {
		return 0, false, false
	}
	id, found := i.byName[key]
	if !found {
		return 0, false, false
	}
	return id, i.ambiguous[key], true
}

// AmbiguousCount es la cantidad de nombres normalizados que matchearon más de un
// ejercicio del mismo dueño.
func (i *NameIndex) AmbiguousCount() int {
	return len(i.ambiguous)
}
