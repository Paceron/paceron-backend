package main

import (
	"fmt"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

// seedMarker viaja en `description` de las filas de catálogo que crea el seed
// (exercises, sessions, training_plans). Es un campo que ninguna pantalla
// muestra y que el cleanup puede leer para distinguir "Distancia 4 km del seed"
// de "Distancia 4 km que ya tenía el entrenador": sin el marcador, un cleanup
// por nombre borraría catálogo ajeno.
const seedMarker = "[seed_dashboard]"

// Claves de los ejercicios del seed. Se escriben explícitas (y no se derivan
// del nombre) porque el generador y el plan de defectos las referencian desde el
// código, y un nombre mal escrito en un string sería un bug silencioso.
const (
	keySparring    = "sparring"
	keyTrote       = "trote"
	keyElongacion  = "elongacion"
	keyRitmo4k     = "ritmo_4k"
	keyRitmo6k     = "ritmo_6k"
	keyRitmo8k     = "ritmo_8k"
	keyRitmo10k    = "ritmo_10k"
	keyRitmo21k    = "ritmo_21k"
	keyPasadas1000 = "pasadas_1000"
	keyPasadas2000 = "pasadas_2000"
	keyMax100      = "max_100"
	keyMax200      = "max_200"
)

func intPtr(v int) *int           { return &v }
func strPtr(v string) *string     { return &v }
func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }

// catalogItem es una fila de `exercises` del seed. Key es la clave estable que
// usan el generador, el plan de defectos y el cleanup: nunca se persiste, solo
// sirve para referirse al ejercicio desde el código.
type catalogItem struct {
	Key         string
	Name        string
	Kind        string
	Minutes     *int
	DistanceM   *int
	MuscleGroup *string
}

// catalog son los 12 ejercicios del seed (tarea 7.2), uno por celda de la
// matriz de tipos vigente:
//
//   - liviano ×2 (Sparring y Trote Vuelta a la Calma): tienen tiempo Y distancia,
//     así que su eje declarado es distance_and_time.
//   - elongacion ×1 (Elongación): tiempo, sin distancia. Lleva muscle_group
//     porque la matriz del front lo exige para este tipo.
//   - ritmo_continuo ×5 (4/6/8/10/21 km): necesitan los dos objetivos.
//   - velocidad_distancia ×4: dos con tiempo (Pasadas 1000/2000) y dos solo con
//     distancia (Velocidad Máxima 100/200 m).
//
// "Calentamiento" se renombra a "Sparring": el rol de un bloque lo define
// session_exercise_instances.role, no el nombre del ejercicio (tarea 7.3).
var catalog = []catalogItem{
	{Key: keySparring, Name: "Sparring", Kind: string(constants.ExerciseKindLiviano), Minutes: intPtr(10), DistanceM: intPtr(1200)},
	{Key: keyTrote, Name: "Trote Vuelta a la Calma", Kind: string(constants.ExerciseKindLiviano), Minutes: intPtr(15), DistanceM: intPtr(1200)},
	{Key: keyElongacion, Name: "Elongación", Kind: string(constants.ExerciseKindElongacion), Minutes: intPtr(10), MuscleGroup: strPtr(string(constants.MuscleGroupGemelos))},

	{Key: keyRitmo4k, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(30), DistanceM: intPtr(4000)},
	{Key: keyRitmo6k, Name: "Distancia 6 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(40), DistanceM: intPtr(6000)},
	{Key: keyRitmo8k, Name: "Distancia 8 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(52), DistanceM: intPtr(8000)},
	{Key: keyRitmo10k, Name: "Distancia 10 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(60), DistanceM: intPtr(10000)},
	{Key: keyRitmo21k, Name: "Distancia 21 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(150), DistanceM: intPtr(21000)},

	{Key: keyPasadas1000, Name: "Pasadas 1000", Kind: string(constants.ExerciseKindVelocidadDistancia), Minutes: intPtr(4), DistanceM: intPtr(1000)},
	{Key: keyPasadas2000, Name: "Pasadas 2000", Kind: string(constants.ExerciseKindVelocidadDistancia), Minutes: intPtr(10), DistanceM: intPtr(2000)},
	// Un sprint de 100 m son ~12 s = 0,2 minutos: no es representable en un INT
	// de minutos (redondeado da 0). Por eso Velocidad Máxima va como
	// velocidad_distancia con `minutes` en null, que sí es exacto y además lo
	// puede crear un entrenador desde el form (tarea 7.2).
	{Key: keyMax100, Name: "Velocidad Máxima 100 m", Kind: string(constants.ExerciseKindVelocidadDistancia), DistanceM: intPtr(100)},
	{Key: keyMax200, Name: "Velocidad Máxima 200 m", Kind: string(constants.ExerciseKindVelocidadDistancia), DistanceM: intPtr(200)},
}

func catalogByKey(key string) (catalogItem, bool) {
	for _, item := range catalog {
		if item.Key == key {
			return item, true
		}
	}
	return catalogItem{}, false
}

// sessionBlock es un bloque de una plantilla de sesión. Repeat es la cantidad de
// series que el corredor reporta de ese bloque.
type sessionBlock struct {
	ExerciseKey string
	Role        string
	Repeat      int
	RestMinutes int
}

type sessionTemplate struct {
	Key    string
	Name   string
	Blocks []sessionBlock
}

func warmupBlock(key string) sessionBlock {
	return sessionBlock{ExerciseKey: key, Role: string(constants.SessionExerciseRoleWarmup), Repeat: 1}
}

func mainBlock(key string, repeat int) sessionBlock {
	return sessionBlock{ExerciseKey: key, Role: string(constants.SessionExerciseRoleMain), Repeat: repeat}
}

func cooldownBlock(key string) sessionBlock {
	return sessionBlock{ExerciseKey: key, Role: string(constants.SessionExerciseRoleCooldown), Repeat: 1}
}

// sessions son las 4 plantillas de sesión del seed. Todas tienen la misma
// estructura (Sparring → Elongación → bloque principal → Vuelta a la calma) y
// se diferencian en el bloque principal: mientras el bloque principal sea menor
// a 8 km puede traer más de un ejercicio, y desde 8 km para arriba es el único
// (tarea 7.4).
var sessions = []sessionTemplate{
	{
		Key:  "volumen",
		Name: "Sesión de volumen",
		Blocks: []sessionBlock{
			warmupBlock(keySparring), warmupBlock(keyElongacion),
			mainBlock(keyRitmo4k, 1), mainBlock(keyRitmo6k, 1),
			cooldownBlock(keyTrote),
		},
	},
	{
		Key:  "velocidad",
		Name: "Sesión de velocidad",
		Blocks: []sessionBlock{
			warmupBlock(keySparring), warmupBlock(keyElongacion),
			mainBlock(keyPasadas1000, 5), mainBlock(keyMax100, 3), mainBlock(keyMax200, 3),
			cooldownBlock(keyTrote),
		},
	},
	{
		Key:  "progresiva",
		Name: "Sesión progresiva",
		Blocks: []sessionBlock{
			warmupBlock(keySparring), warmupBlock(keyElongacion),
			mainBlock(keyRitmo8k, 1),
			cooldownBlock(keyTrote),
		},
	},
	{
		Key:  "larga",
		Name: "Sesión larga",
		Blocks: []sessionBlock{
			warmupBlock(keySparring), warmupBlock(keyElongacion),
			mainBlock(keyRitmo21k, 1),
			cooldownBlock(keyTrote),
		},
	},
}

// planTemplate es una semana del plan de 14 días. El stamp mapea sequence_no
// 1..7 a los 7 días consecutivos desde start_date, así que el martes es el
// sequence_no 2 y el jueves el 4 (tarea 7.5: 2 sesiones presenciales por semana,
// el resto se saltea con exclude_dates).
type planTemplate struct {
	Key            string
	Name           string
	Tuesday        string // sessionTemplate.Key
	Thursday       string // sessionTemplate.Key
	PresencialFrom string
	PresencialTo   string
}

// plans son las dos semanas del seed: 4 días de calendario en total.
var plans = []planTemplate{
	{Key: "semana_1", Name: "Semilla dashboard · Semana 1", Tuesday: "velocidad", Thursday: "volumen", PresencialFrom: "18:00", PresencialTo: "20:00"},
	{Key: "semana_2", Name: "Semilla dashboard · Semana 2", Tuesday: "progresiva", Thursday: "larga", PresencialFrom: "08:00", PresencialTo: "11:00"},
}

// sessionKeyForSeq devuelve la plantilla de sesión del día de la semana, o ""
// si ese día es descanso.
func (p planTemplate) sessionKeyForSeq(seq int) string {
	switch seq {
	case planDayTuesday:
		return p.Tuesday
	case planDayThursday:
		return p.Thursday
	}
	return ""
}

// planDayRequest arma los 7 PlanDayRequest de una semana: martes y jueves son de
// entrenamiento (presenciales, con horario y lugar), el resto descanso.
// sessionIDs traduce la clave de la plantilla al id de `sessions`.
func (p planTemplate) planDayRequest(sessionIDs map[string]int64) []trainingplan.PlanDayRequest {
	location := trainingplan.Location{Lat: -34.6037, Lng: -58.3816, Label: strPtr("Parque Sarmiento")}
	days := make([]trainingplan.PlanDayRequest, 0, planDaysPerWeek)
	for seq := 1; seq <= planDaysPerWeek; seq++ {
		day := trainingplan.PlanDayRequest{SequenceNo: seq, Kind: string(constants.PlanDayKindRest)}
		if key := p.sessionKeyForSeq(seq); key != "" {
			day.Kind = string(constants.PlanDayKindTraining)
			day.SessionID = int64Ptr(sessionIDs[key])
			day.DefaultPresencial = boolPtr(true)
			day.DefaultTimeFrom = strPtr(p.PresencialFrom)
			day.DefaultTimeTo = strPtr(p.PresencialTo)
			day.DefaultLocation = &location
		}
		days = append(days, day)
	}
	return days
}

func int64Ptr(v int64) *int64 { return &v }

// seedDescription es lo que se escribe en `description` de cada fila de
// catálogo que crea el seed. Lleva el nombre para que un humano que encuentre
// la fila entienda de dónde salió.
func seedDescription(what string) *string {
	return strPtr(fmt.Sprintf("%s Creado por el seed del dashboard analítico (%s). No borrar a mano.", seedMarker, what))
}
