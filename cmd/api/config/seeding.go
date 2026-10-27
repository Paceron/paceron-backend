package config

import (
	"fmt"
	"os"
	"strings"
)

// seedingEnabledFlag habilita el stamping de histórico vía API, que existe solo
// para el seed del dashboard. Sin ella no hay forma de cargar datos en fechas
// pasadas: isCalendarDayClosed cierra toda fecha anterior a hoy.
const seedingEnabledFlag = "SEEDING_ENABLED"

// Seeding es el valor de SEEDING_ENABLED leído al cargar la config. Se expone
// como var (y no como lectura directa de la env var en cada uso) para que los
// tests puedan prenderla sin depender del orden de carga de la config.
var Seeding bool

func loadSeedingConfig() {
	Seeding = envBoolDefaultFalse(seedingEnabledFlag)
}

// SeedingEnabled indica si el stamping de histórico está habilitado. Nunca se
// lee del body de la request: es una decisión del servidor, no del cliente.
func SeedingEnabled() bool {
	return Seeding
}

// AssertSeedingStage aborta el arranque cuando SEEDING_ENABLED está activo sin
// --stage=local (design.md D14). El panic es a propósito: un seed que corre a
// medias contra un entorno compartido es peor que un proceso que no levanta.
//
// La guarda NO es "si falta cualquiera de las dos, no arrancar": con el flag en
// false —el estado normal de todos los deploys— no hay ninguna comprobación, así
// que el proceso arranca siempre. Tampoco sirve IsProduction() como contraparte:
// staging no contiene "prod" en la env var y es un entorno real que comparte el
// proyecto de Supabase de testing.
func AssertSeedingStage() {
	if !SeedingEnabled() {
		return
	}
	if IsLocalStage() {
		return
	}
	panic(fmt.Sprintf(
		"SEEDING_ENABLED=true está activo pero el proceso no arrancó con --stage=local (stage detectado: %s). "+
			"El stamping de histórico queda deshabilitado: corré el backend con --stage=local o sacá la variable.",
		ResolvedStage(),
	))
}

// envBoolDefaultFalse es el espejo de envBoolDefaultTrue: sin la variable, o con
// un valor que no sea un "sí" explícito, queda false. La inactividad es el
// default a propósito — es lo que hace que AssertSeedingStage sea inocuo.
func envBoolDefaultFalse(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// ResolvedStage es el nombre del stage que el proceso está usando, para
// mensajes de error que tienen que ser accionables sin abrir el código.
func ResolvedStage() string {
	switch {
	case IsLocalStage():
		return "LOCAL"
	case IsProductionStage():
		return "PRODUCTION"
	default:
		return "testing"
	}
}