package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setSeeding fuerza el estado de la bandera como lo vería el proceso ya
// arrancado, sin tocar la env var ni el ciclo de carga de la config.
func setSeeding(t *testing.T, value bool) {
	t.Helper()
	previous := Seeding
	Seeding = value
	t.Cleanup(func() { Seeding = previous })
}

// withStageFlag simula el argv del proceso: IsLocalStage/IsProductionStage leen
// os.Args, así que sin esto la rama del panic no se puede ejercitar.
func withStageFlag(t *testing.T, args ...string) {
	t.Helper()
	previous := os.Args
	os.Args = append([]string{"paceron-backend"}, args...)
	t.Cleanup(func() { os.Args = previous })
}

func TestSeedingConfig_InertePorDefault(t *testing.T) {
	// Sin la variable, y con cualquier valor que no sea un sí explícito, la
	// bandera queda apagada: es lo que hace inocuo al assert de arranque.
	for _, value := range []string{"", "false", "0", "no", "FALSE", "quizás", " "} {
		t.Setenv(seedingEnabledFlag, value)
		loadSeedingConfig()
		assert.False(t, SeedingEnabled(), "SEEDING_ENABLED=%q no debe habilitar el seeding", value)
	}
}

func TestSeedingConfig_SoloValoresVerdadososHabilitan(t *testing.T) {
	for _, value := range []string{"true", "TRUE", " True ", "1", "yes"} {
		t.Setenv(seedingEnabledFlag, value)
		loadSeedingConfig()
		assert.True(t, SeedingEnabled(), "SEEDING_ENABLED=%q debe habilitar el seeding", value)
	}
}

func TestAssertSeedingStage_BanderaInerteNoComprueba(t *testing.T) {
	// Regresión del modo de falla caro: con la bandera apagada NO hay
	// comprobación de stage aunque el proceso no esté en local. Implementarlo
	// como "si falta cualquiera de las dos, no arrancar" deja el backend caído
	// en todos los deploys.
	setSeeding(t, false)
	withStageFlag(t)

	assert.NotPanics(t, AssertSeedingStage)
}

func TestAssertSeedingStage_LocalConBanderaActivaArranca(t *testing.T) {
	setSeeding(t, true)
	withStageFlag(t, "--stage=local")

	assert.NotPanics(t, AssertSeedingStage)
}

func TestAssertSeedingStage_FueraDeLocalConBanderaActivaPanica(t *testing.T) {
	for _, args := range [][]string{
		{},                        // sin flag: testing cloud
		{"--stage=production"},    // producción
		{"--stage=anything-else"}, // environment nuevo, no registrado
	} {
		setSeeding(t, true)
		withStageFlag(t, args...)

		assert.PanicsWithValue(t,
			"SEEDING_ENABLED=true está activo pero el proceso no arrancó con --stage=local (stage detectado: "+ResolvedStage()+"). "+
				"El stamping de histórico queda deshabilitado: corré el backend con --stage=local o sacá la variable.",
			AssertSeedingStage, "args=%v", args)
	}
}

// El mensaje del panic tiene que nombrar el stage detectado: sin eso el error no
// es accionable sin abrir el código (design.md D14).
func TestAssertSeedingStage_MensajeNombraElStage(t *testing.T) {
	setSeeding(t, true)
	withStageFlag(t)

	panicked := func() (msg string) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg, _ = r.(string)
		}()
		AssertSeedingStage()
		return ""
	}()

	assert.Contains(t, panicked, "stage detectado: testing")
}

// IsProduction() no alcanza como guarda del seeding: staging no contiene "prod"
// en la env var y es un entorno desplegado real.
func TestAssertSeedingStage_StagingNoEsProduccion(t *testing.T) {
	t.Setenv(_env, "staging")
	setSeeding(t, true)
	withStageFlag(t)

	assert.False(t, IsProduction(), "staging no es producción para config.IsProduction")
	assert.Panics(t, AssertSeedingStage)
}

func TestResolvedStage(t *testing.T) {
	withStageFlag(t)
	assert.Equal(t, "testing", ResolvedStage())

	withStageFlag(t, "--stage=local")
	assert.Equal(t, "LOCAL", ResolvedStage())

	withStageFlag(t, "--stage=production")
	assert.Equal(t, "PRODUCTION", ResolvedStage())
}
