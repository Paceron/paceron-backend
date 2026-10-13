// Package gametime centraliza TODA la noción de tiempo de gamificación
// (design.md gamificacion-core D1): la TZ America/Argentina/Cordoba es fija y
// la resolvemos una sola vez en init. El "reloj" (Now) es una var global
// sobreescribible: los tests deterministas la reemplazan con time.Date y así
// el resultado no depende de la hora real del runner.
package gametime

import (
	"time"

	// Garantiza que LoadLocation resuelva Cordoba aunque el host/imagen no
	// tenga tzdata propia.
	_ "time/tzdata"
)

const gamificationTZ = "America/Argentina/Cordoba"

// Location es la TZ de gamificación. Todos los cálculos de frontera de día
// (ventana, ayer, hoy) se evalúan acá, nunca en la TZ local del servidor.
var Location = newGamificationLocation()

// Now es el reloj de gamificación. En producción es el wall clock; los tests
// lo pisan (con t.Cleanup para restaurar).
var Now = func() time.Time { return time.Now() }

func newGamificationLocation() *time.Location {
	loc, err := time.LoadLocation(gamificationTZ)
	if err != nil {
		panic("gametime: no se pudo resolver la TZ " + gamificationTZ + ": " + err.Error())
	}
	return loc
}

// DateOnly colapsa un instante a su medianoche en la TZ de gamificación.
func DateOnly(t time.Time) time.Time {
	n := t.In(Location)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, Location)
}

// Today es la medianoche del día de hoy según la TZ de gamificación.
func Today() time.Time { return DateOnly(Now()) }

// Yesterday es la medianoche del día de ayer según la TZ de gamificación.
func Yesterday() time.Time { return Today().AddDate(0, 0, -1) }
