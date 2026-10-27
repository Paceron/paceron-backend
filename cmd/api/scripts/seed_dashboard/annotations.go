package main

import (
	"fmt"
	"math/rand"
	"strconv"

	"simple-arq-golang/cmd/api/domains/constants"
)

// annotationFor arma el texto de annotations de una serie. El feedback contextual
// es lo que hace que la pantalla de revisión se pueda leer (tarea 7.9): un "Buen
// trabajo" repetido en todas las filas es indistinguible de un texto genérico, y
// el corredor no le puede contar nada al entrenador.
//
// El cuerpo se elige por (tipo de ejercicio, si cumplió el objetivo o no) y se
// completa con los datos reales de la serie (ritmo, frecuencia, pulsación y
// esfuerzo), así que dos corredores no reciben el mismo texto en la misma sesión.
func annotationFor(rng *rand.Rand, in seriesInput, missed bool, avgHR int16, rpe int16) string {
	pool := annotationPool(in.Exercise.Kind, missed)
	body := pool[rng.Intn(len(pool))]
	return fmt.Sprintf("%s Ritmo %s, FC %d, RPE %d.",
		body, paceLabel(in.Exercise), int(avgHR), int(rpe))
}

// paceLabel arma el ritmo en min/km desde el objetivo del ejercicio. Cuando el
// ejercicio no tiene objetivo de tiempo (Velocidad maxima 100/200 m) usa el
// ritmo de referencia del generador, que es el único dato disponible.
func paceLabel(exercise catalogItem) string {
	if exercise.DistanceM == nil || *exercise.DistanceM <= 0 {
		return "no aplica"
	}
	minutes := 0.0
	if exercise.Minutes != nil && *exercise.Minutes > 0 {
		minutes = float64(*exercise.Minutes)
	} else {
		minutes = paceSecPer100m * float64(*exercise.DistanceM) / 100 / 60
	}
	if minutes <= 0 {
		return "no aplica"
	}
	perKm := minutes / (float64(*exercise.DistanceM) / 1000)
	return strconv.FormatFloat(perKm, 'f', 2, 64) + " min/km"
}

// annotationPool devuelve los cuerpos de texto disponibles para un tipo de
// ejercicio según si la serie cumplió el objetivo o no. Están escritos uno por
// uno a propósito: los que se arman por concatenación ("Buen trabajo en X") se
// leen igual en cincuenta filas y no aportan nada.
func annotationPool(kind string, missed bool) []string {
	if missed {
		if pool, ok := missedAnnotations[kind]; ok {
			return pool
		}
	} else if pool, ok := hitAnnotations[kind]; ok {
		return pool
	}
	return annotationKindFallback
}

var hitAnnotations = map[string][]string{
	string(constants.ExerciseKindLiviano): {
		"Sali bien las primeras vueltas y afloje al final para no llegar roto a la elongacion.",
		"Ritmo conversado casi toda la sesion, que era justamente lo que pedia el bloque.",
		"Me costo arrancar por el frio y despues quedo parejo de principio a fin.",
		"Volvi en el ultimo minuto porque venía mas rapido de lo que marcaba el reloj.",
	},
	string(constants.ExerciseKindElongacion): {
		"Estiramientos despacio, sobre todo isquiotibiales, que vinieron cargados de la semana.",
		"Deje cada postura el tiempo que corresponde y no me apure: fue la parte que mas me sirvio.",
		"El isquio lo senti mucho mas suelto que en la sesion anterior.",
		"Rodillas sin molestia, algo que la semana pasada ya me estaba avisando.",
	},
	string(constants.ExerciseKindRitmoContinuo): {
		"Marque el primer kilómetro conservador y despues subi a un ritmo que podia sostener.",
		"Baje en las bajadas y aguante el ritmo en las subidas, que era lo que servia para hoy.",
		"Tercera vuelta con el tiempo justo; la ultima la hice con sensacion de piernas pesadas.",
		"Se me activo el hombro al final y afloje el ritmo para no empeorar.",
	},
	string(constants.ExerciseKindVelocidadDistancia): {
		"Salidas limpias y las ultimas repeticiones todavia con la misma tecnica.",
		"Busque que la desaceleracion fuera suave entre pasada y pasada.",
		"Tecnica bastante solida, salvo la ultima, que me salio mas encogida que las demas.",
		"El reloj se trabo en la cuarta y segui igual, asi que el tiempo de esa no es confiable.",
	},
}

var missedAnnotations = map[string][]string{
	string(constants.ExerciseKindLiviano): {
		"Me pase de largo: la ultima vuelta la hice mas rapido de lo previsto y despues llego tirado.",
		"Anduve bien hasta el final y ahi se me vino el tiempo encima.",
		"Sali demasiado conservador y no busque ritmo en ningun momento de la vuelta.",
	},
	string(constants.ExerciseKindElongacion): {
		"Me quede corto en los estiramientos: se me hizo tarde y los apure.",
		"Salte isquiotibiales por la molestia en la espalda baja que arrastro de la semana.",
		"Estire rapido porque tenia que salir a trabajar apenas termine.",
	},
	string(constants.ExerciseKindRitmoContinuo): {
		"Corte el ultimo kilómetro porque el hombro empezo a molestar y no queria forzar.",
		"Sali demasiado rapido, me fundi en el medio y la segunda mitad la reme con pura ganas.",
		"Hubo viento en contra de ida y a favor de vuelta; el ritmo quedo lejos del objetivo.",
		"Pare dos veces por la fuente de la plaza y el tiempo final se me fue de las manos.",
	},
	string(constants.ExerciseKindVelocidadDistancia): {
		"Las dos primeras salidas fueron conservadoras y despues no llegue al objetivo en ninguna.",
		"Me distribute mal entre pasada y pasada, y las recuperaciones fueron larguisimas.",
		"Zafe la ultima, pero con tecnica rota: la cadera se me caia en cada pisada.",
		"Me distraje hablando con un companero del grupo y las dos ultimas salieron mas lentas.",
	},
}

// annotationKindFallback son los cuerpos que usan los tipos sin entrada propia
// (un kind legacy, si algun dia el catalogo trae uno).
var annotationKindFallback = []string{
	"Ritmo sostenido todo el bloque, sin sobresaltos ni cambios de paso.",
	"Sali con las piernas frescas y lo cerre bien.",
}
