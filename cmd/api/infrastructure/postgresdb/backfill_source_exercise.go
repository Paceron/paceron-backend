package postgresdb

import (
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"

	"gorm.io/gorm"
)

// BackfillSourceExerciseID puebla exercise_instances.source_exercise_id en las
// filas anteriores al change instancia-referencia-catalogo (8670ffe, 2026-09-21),
// que quedaron en NULL porque AutoMigrate agrega la columna sin tocar datos.
//
// El match es por (owner_id, nombre normalizado), no por nombre global:
// `exercises` es catálogo por dueño, así que dos entrenadores pueden tener
// "Distancia 4 km" con objetivos distintos. El nombre se normaliza en Go y no
// en SQL a propósito: `unaccent` es una extensión que no está garantizada en el
// Postgres del despliegue, y la función ya existe testeada.
//
// Idempotente por construcción: el WHERE filtra las NULL, así que una segunda
// pasada no encuentra nada que hacer. Es un ítem más del bloque numerado, con la
// misma reversing que el 9.
//
// Lo que NO matchea se deja en NULL a propósito: el fallback por nombre de
// /exercises/{id}/series lo resuelve en tiempo de query, y esas filas se cuentan
// en data_quality.legacy_sessions para que el caso sea visible y no un
// descartamiento silencioso.
// Exportada para poder verificarla en aislamiento desde el package de test
// externo (`postgresdb_test`): el test interno no puede importar `testutils`
// porque `testutils` importa este package, y Go no permite ese ciclo.
func BackfillSourceExerciseID(db *gorm.DB) error {
	type unlinked struct {
		InstanceID int64
		Name       string
		OwnerID    int64
	}

	// Solo las instancias sin link, del scope completo: el backfill no depende de
	// qué equipos le importen a este proceso.
	var rows []unlinked
	err := db.Raw(`
		SELECT DISTINCT ei.id AS instance_id, ei.name AS name, t.owner_id AS owner_id
		FROM exercise_instances ei
		JOIN session_exercise_instances sei ON sei.exercise_instance_id = ei.id
		JOIN group_calendar_days gcd       ON gcd.session_instance_id = sei.session_instance_id
		JOIN groups g  ON g.id = gcd.group_id
		JOIN teams t   ON t.id = g.team_id
		WHERE ei.source_exercise_id IS NULL
	`).Scan(&rows).Error
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	type catalogRow struct {
		ID      int64
		OwnerID int64
		Name    string
	}
	var catalog []catalogRow
	if err := db.Raw(`SELECT id, owner_id, name FROM exercises WHERE deleted_at IS NULL`).Scan(&catalog).Error; err != nil {
		return err
	}

	// Índice por dueño: (owner_id, nombre normalizado) -> candidatos, para no
	// recorrer el catálogo entero por cada instancia.
	byOwner := map[int64]map[string][]int64{}
	for _, c := range catalog {
		key := dashboard.NormalizeExerciseName(c.Name)
		if byOwner[c.OwnerID] == nil {
			byOwner[c.OwnerID] = map[string][]int64{}
		}
		byOwner[c.OwnerID][key] = append(byOwner[c.OwnerID][key], c.ID)
	}

	var filled, ambiguous, unmatched int
	for _, r := range rows {
		candidates := byOwner[r.OwnerID][dashboard.NormalizeExerciseName(r.Name)]
		switch len(candidates) {
		case 0:
			unmatched++
		case 1:
			// Un solo candidato: es el match. Se resuelve por id para que la
			// ejecución sea idempotente aunque el catálogo cambie después.
			stmt := db.Exec(
				`UPDATE exercise_instances SET source_exercise_id = ? WHERE id = ? AND source_exercise_id IS NULL`,
				candidates[0], r.InstanceID,
			)
			if stmt.Error != nil {
				return stmt.Error
			}
			filled += int(stmt.RowsAffected)
		default:
			// Más de un ejercicio del mismo dueño con el mismo nombre: no se
			// elige al azar. Se deja en NULL y el fallback por nombre lo resuelve
			// por exercises.id ascendente en tiempo de query (tarea 4.1b).
			ambiguous++
		}
	}

	customlogger.Warnf(nil,
		"backfill source_exercise_id: %d llenadas, %d ambiguas (mismo nombre, mismo dueño), %d sin match (quedan en NULL y las resuelve el fallback por nombre)",
		filled, ambiguous, unmatched,
	)
	return nil
}
