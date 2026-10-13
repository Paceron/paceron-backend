package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeExerciseName(t *testing.T) {
	cases := map[string]string{
		"Distancia 4 km":           "distancia 4 km",
		"DISTANCIA 4 KM":           "distancia 4 km",
		"distancia 4 km.":          "distancia 4 km",
		"  Distancia   4  km  ":    "distancia 4 km",
		"Ritmo Continuo":           "ritmo continuo",
		"ritmo-continuo":           "ritmo continuo",
		"ritmo_continuo":           "ritmo continuo",
		"Ritmo — Continuo":         "ritmo continuo",
		"Elongación":               "elongacion",
		"Pasadas 1000 m":           "pasadas 1000 m",
		"Velocidad Máxima (100 m)": "velocidad maxima 100 m",
		"Trote Vuelta a la Calma":  "trote vuelta a la calma",
		"Ñandú / ñoqui":            "nandu noqui",
		"":                         "",
		"!!!":                      "",
		"4x(800m) + 400m，取":        "4x 800m 400m 取",
	}
	for input, expected := range cases {
		assert.Equal(t, expected, NormalizeExerciseName(input), "input=%q", input)
	}
}

// La normalización tiene que ser idempotente: el DAO resuelve por nombre
// normalizado y el resultado se vuelve a comparar en la capa de arriba.
func TestNormalizeExerciseName_Idempotente(t *testing.T) {
	for _, name := range []string{"Distancia 4 km", "Ritmo-Continuo", "Elongación", "Pasadas 2000 m"} {
		once := NormalizeExerciseName(name)
		assert.Equal(t, once, NormalizeExerciseName(once), "name=%q", name)
	}
}

func TestNameIndex_ResuelvePorNombreNormalizado(t *testing.T) {
	index := NewNameIndex([]CatalogExercise{
		{ID: 7, OwnerID: 1, Name: "Distancia 4 km"},
		{ID: 8, OwnerID: 1, Name: "Distancia 21 km"},
	})

	id, ambiguous, ok := index.Resolve(1, "distancia 4 KM.")
	require.True(t, ok)
	assert.Equal(t, int64(7), id)
	assert.False(t, ambiguous)

	_, _, ok = index.Resolve(1, "Trote livre")
	assert.False(t, ok, "un nombre que no está en el catálogo no resuelve")
}

// El fallback por nombre se acota al mismo dueño: `exercises` es catálogo por
// dueño, así que sin owner_id el dashboard mostraría el objetivo de un
// entrenador evaluando las sesiones de otro.
func TestNameIndex_NoCruzaDuenos(t *testing.T) {
	index := NewNameIndex([]CatalogExercise{
		{ID: 7, OwnerID: 1, Name: "Distancia 4 km"},
		{ID: 20, OwnerID: 2, Name: "Distancia 4 km"},
	})

	id, _, ok := index.Resolve(1, "Distancia 4 km")
	require.True(t, ok)
	assert.Equal(t, int64(7), id)

	id, _, ok = index.Resolve(2, "Distancia 4 km")
	require.True(t, ok)
	assert.Equal(t, int64(20), id)

	_, _, ok = index.Resolve(3, "Distancia 4 km")
	assert.False(t, ok, "un dueño sin ese ejercicio no resuelve")
}

// Nombre duplicado del mismo dueño: resuelve al id más bajo y queda marcado como
// ambiguo, para que el caso sea visible en vez de depender del orden de la query.
func TestNameIndex_DuplicadoDelMismoDueñoResuelveAlIdMasBajo(t *testing.T) {
	index := NewNameIndex([]CatalogExercise{
		{ID: 30, OwnerID: 1, Name: "Distancia 4 km"},
		{ID: 12, OwnerID: 1, Name: "distancia 4 km."},
		{ID: 44, OwnerID: 1, Name: "Distancia 21 km"},
	})

	id, ambiguous, ok := index.Resolve(1, "DISTANCIA 4 KM")
	require.True(t, ok)
	assert.Equal(t, int64(12), id, "el id ascendente gana, sea cual sea el orden de entrada")
	assert.True(t, ambiguous)
	assert.Equal(t, 1, index.AmbiguousCount())

	_, ambiguous, ok = index.Resolve(1, "Distancia 21 km")
	require.True(t, ok)
	assert.False(t, ambiguous, "un nombre único no es ambiguo")
	assert.Equal(t, 1, index.AmbiguousCount())
}

func TestNameIndex_OrdenDeEntradaNoCambiaElResultado(t *testing.T) {
	forwards := NewNameIndex([]CatalogExercise{
		{ID: 30, OwnerID: 1, Name: "Distancia 4 km"},
		{ID: 12, OwnerID: 1, Name: "distancia 4 km."},
	})
	backwards := NewNameIndex([]CatalogExercise{
		{ID: 12, OwnerID: 1, Name: "distancia 4 km."},
		{ID: 30, OwnerID: 1, Name: "Distancia 4 km"},
	})

	idA, ambiguousA, okA := forwards.Resolve(1, "Distancia 4 km")
	idB, ambiguousB, okB := backwards.Resolve(1, "Distancia 4 km")
	require.True(t, okA)
	require.True(t, okB)
	assert.Equal(t, idA, idB)
	assert.Equal(t, ambiguousA, ambiguousB)
}

func TestNameIndex_NombreVacioNoResuelve(t *testing.T) {
	index := NewNameIndex([]CatalogExercise{{ID: 7, OwnerID: 1, Name: "   ??  "}, {ID: 8, OwnerID: 1, Name: "Trote"}})

	_, _, ok := index.Resolve(1, "!!!")
	assert.False(t, ok, "un nombre que normaliza a vacío no puede matchear contra otro vacío")
	assert.Equal(t, 0, index.AmbiguousCount())

	id, _, ok := index.Resolve(1, "trote")
	require.True(t, ok)
	assert.Equal(t, int64(8), id)
}
