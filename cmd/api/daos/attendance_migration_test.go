package daos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migrationScript es el archivo que hay que correr a mano en cada base. Ver el
// header del archivo: NO lo aplica AutoMigrate.
const migrationScript = "../../../scripts/migrate_attendance_source_provenance.sql"

func readMigrationScript(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Clean(migrationScript))
	require.NoError(t, err, "el script de migración no se encuentra en %s", migrationScript)
	return string(content)
}

// TestAttendanceMigration_ScriptDeclaresSessionInstanceForeignKey reemplaza al
// test de COMPORTAMIENTO de la FK que hubo antes ("insertar con
// training_session_id inexistente debe fallar").
//
// Por qué no puede ser un test de comportamiento: en este proyecto los modelos
// GORM no declaran asociaciones `constraint:`, así que AutoMigrate no genera
// ninguna foreign key. Una base creada por AutoMigrate —que es exactamente lo que
// hace CI— queda sin la restricción, y el insert con una sesión inexistente
// entra sin error. El test fallaría siempre, y "siempre rojo" no es un test: es
// ruido que entrena al equipo a ignorar la suite.
//
// Lo que sí es verificable, y es lo que importa, es que el SQL siga en el repo.
// Este test falla si alguien borra el script, lo deja sin la constraint, o le
// cambia el nombre. Para verificar el COMPORTAMIENTO hay que correr el script
// contra una base migrada y hacer el insert a mano — es el paso 5.7 del change.
func TestAttendanceMigration_ScriptDeclaresSessionInstanceForeignKey(t *testing.T) {
	content := readMigrationScript(t)

	t.Run("declara la constraint con el nombre esperado", func(t *testing.T) {
		assert.Contains(t, content, "fk_attendances_session_instance",
			"el nombre de la constraint es parte del contrato: el paso 6 de verificación la busca por nombre")
	})

	t.Run("la constraint no lleva ON DELETE", func(t *testing.T) {
		// CASCADE borraría en silencio el histórico de asistencias de un corredor
		// si alguna vez se expone el borrado de una sesión. RESTRICT falla visible.
		//
		// Se aísla la sentencia ADD CONSTRAINT en vez de buscar la cadena en todo
		// el archivo: el comentario de arriba explica por qué NO hay CASCADE, así
		// que un grep sobre el archivo entero pasaría siempre y no probaría nada.
		idx := strings.Index(content, "ADD CONSTRAINT fk_attendances_session_instance")
		require.NotEqual(t, -1, idx, "no se encontró la sentencia ADD CONSTRAINT")

		stmt := content[idx:]
		if end := strings.Index(stmt, ";"); end != -1 {
			stmt = stmt[:end]
		}
		assert.NotContains(t, strings.ToUpper(stmt), "ON DELETE",
			"la constraint no debe declarar ON DELETE: CASCADE borraría el histórico "+
				"de asistencias en silencio. RESTRICT (el default) falla de forma visible")
	})

	t.Run("agrega las dos columnas de provenance", func(t *testing.T) {
		assert.Contains(t, content, "ADD COLUMN IF NOT EXISTS source")
		assert.Contains(t, content, "ADD COLUMN IF NOT EXISTS registered_by_user_id")
	})

	t.Run("el backfill es qr y no manual", func(t *testing.T) {
		// El único escritor previo al change era el registro por QR, así que todo
		// lo histórico es qr. Si alguien "corrige" esto a manual, todas las
		// asistencias del QR quedan mal marcadas y no hay forma de distinguirlas.
		assert.Contains(t, content, "SET source = 'qr'")
		assert.NotContains(t, content, "SET source = 'manual'")
	})

	t.Run("es idempotente en los pasos que agregan objetos", func(t *testing.T) {
		// Sin esto, correr el script dos veces (normal en un deploy rehecho)
		// revienta con "already exists" a la mitad.
		assert.Contains(t, content, "ADD COLUMN IF NOT EXISTS")
		assert.Contains(t, content, "IF NOT EXISTS (\n    SELECT 1 FROM pg_constraint")
		assert.Contains(t, content, "IF NOT EXISTS (\n    SELECT 1 FROM pg_indexes")
	})

	t.Run("el preview de huerfanas está antes del DELETE", func(t *testing.T) {
		preview := strings.Index(content, "LEFT JOIN session_instances")
		del := strings.Index(content, "DELETE FROM attendances")
		require.NotEqual(t, -1, preview, "falta el preview de las filas huerfanas")
		require.NotEqual(t, -1, del, "falta el DELETE de huerfanas")
		assert.Less(t, preview, del,
			"el SELECT de preview tiene que estar antes del DELETE: es el paso que avisa cuántos datos se borran")
	})
}

// TestAttendanceMigration_ElModeloNoDeclaraLaForeignKey documenta la limitación
// que justifica todo lo de arriba.
//
// Es un test sobre el código, no sobre la base: falla si alguien agrega la
// asociación GORM `constraint:` al modelo. Y eso NO es un error a arreglar — es la
// señal de que la migración manual quedó obsoleta y hay que borrar
// scripts/migrate_attendance_source_provenance.sql junto con este test.
//
// Está para que la decisión quede escrita en el código y no viva solo en una
// conversación: el próximo que lea "Attendance tiene training_session_id" y
// asuma que por lo tanto hay FK va a perder una tarde.
func TestAttendanceMigration_ElModeloNoDeclaraLaForeignKey(t *testing.T) {
	model, err := os.ReadFile(filepath.Clean("../domains/dbs/attendance.go"))
	require.NoError(t, err)

	assert.NotContains(t, string(model), "constraint:",
		"el modelo ahora declara la FK: la migración manual quedó obsoleta. "+
			"Borrar scripts/migrate_attendance_source_provenance.sql, este test y "+
			"TestAttendanceMigration_ScriptDeclaresSessionInstanceForeignKey, y cambiar "+
			"la task 0.8 por un test de comportamiento contra la base")
}
