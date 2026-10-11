package daos

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type GroupCalendarDaoInterface interface {
	Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	// FindByGroupAndDates trae los días existentes para las fechas indicadas
	// en UN solo query (design.md D7, bulk-clear). Espejo en lote de
	// FindByGroupAndDate: devuelve solo las fechas con fila (las ausentes ni
	// aparecen). Sin fechas no toca la DB.
	FindByGroupAndDates(ctx *gin.Context, groupID int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)
	FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	// FindPresencialForGroupsInRange devuelve los días training+presencial de
	// los grupos indicados en las fechas indicadas (base de la detección de
	// colisiones presenciales, design.md D3).
	FindPresencialForGroupsInRange(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)
	// FindForGroupsInRange devuelve todos los días de calendario de los
	// grupos indicados en el rango de fechas, ordenados por fecha (base del
	// calendario agregado, design.md D8).
	FindForGroupsInRange(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	Delete(ctx *gin.Context, groupID int64, date time.Time) error
	DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error
	// FindNextForGroupsByKind devuelve el día más próximo del kind indicado
	// entre los grupos dados con el filtro "hoy cuenta" del banner (design.md
	// D6): date > hoy, o date == hoy solo si no es presencial o el horario
	// presencial todavía no arrancó (mismo criterio que isCalendarDayClosed).
	// nowHHMM es la hora actual en "HH:MM"; today es la medianoche local.
	FindNextForGroupsByKind(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error)
	// FindNextPresencialForGroups devuelve el próximo día training+presencial
	// entre los grupos dados con el mismo filtro "hoy cuenta" del banner
	// (design.md D7): date > hoy, o date == hoy solo si el horario presencial
	// todavía no arrancó. nowHHMM es la hora actual en "HH:MM"; today es la
	// medianoche local.
	FindNextPresencialForGroups(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error)
	ClearSourcePlan(ctx *gin.Context, planID int64) error
	UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
	// FindBySessionInstanceID devuelve el día de calendario que referencia
	// esa session instance (design.md Gap 26 D6, relación 1:1 por diseño):
	// nil si no hay día — instancia huérfana o id inexistente.
	FindBySessionInstanceID(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error)
	// SetPresencialOpenedAt marca la apertura presencial del día solo si
	// presencial_opened_at está NULL (guard SQL idempotente, D5 sin reopen).
	// Devuelve true si esta llamada hizo la apertura real.
	SetPresencialOpenedAt(ctx *gin.Context, dayID int64, at time.Time) (bool, error)
	// SetPresencialClosedAt marca el cierre presencial del día solo si
	// presencial_closed_at está NULL (final, sin reopen). true si esta
	// llamada hizo el cierre real.
	SetPresencialClosedAt(ctx *gin.Context, dayID int64, at time.Time) (bool, error)
	// UpsertMany aplica el mismo upsert (group_id, date) de Upsert sobre un
	// lote en un solo round trip (design.md D5, write path de stamp/bulk):
	// misma fila de SETs, misma limpieza presencial Gap-26 (opened/closed a
	// NULL si la escritura cambia la instancia o apaga is_presencial).
	// RETURNING no garantiza orden: los resultados vuelven igualados a la
	// posición del input por (group_id, date); los callers conservan el
	// resto de sus filas tal cual la construyeron (shape de respuestas
	// idéntico al de hoy).
	UpsertMany(ctx *gin.Context, days []*dbs.GroupCalendarDay) ([]GroupCalendarDayUpsertResult, error)
}

// GroupCalendarDayUpsertResult es la porción de fila que el caller necesita
// del RETURNING de UpsertMany (TO_CHAR del date evita el TZ-shift al
// reconstruir la clave).
type GroupCalendarDayUpsertResult struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type groupCalendarDayDao struct {
	DB *gorm.DB
}

func NewGroupCalendarDayDao(database *gorm.DB) GroupCalendarDaoInterface {
	return &groupCalendarDayDao{DB: database}
}

// Upsert crea o reemplaza el contenido del día (group_id, date) — no hay
// soft-delete acá, DELETE vacía el día físicamente (spec §4).
func (d *groupCalendarDayDao) Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
	var existing dbs.GroupCalendarDay
	err := d.DB.Where("group_id = ? AND date = ?", day.GroupID, day.Date).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("error finding calendar day: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		return d.DB.Create(day).Error
	}
	day.ID = existing.ID
	updates := map[string]interface{}{
		"kind":                 day.Kind,
		"other_name":           day.OtherName,
		"session_instance_id":  day.SessionInstanceID,
		"cancelled_reason":     day.CancelledReason,
		"is_presencial":        day.IsPresencial,
		"presencial_time_from": day.PresencialTimeFrom,
		"presencial_time_to":   day.PresencialTimeTo,
		"presencial_location":  day.PresencialLocation,
		"source_plan_id":       day.SourcePlanID,
	}
	// Gap 26 D5: el estado presencial existe solo mientras el día lo declare.
	// Si la escritura apaga is_presencial (o la reasignación cambia la
	// instancia), opened/closed se limpian: re-activar presencial arranca sin
	// apertura (gate 409 hasta que el owner vuelva a abrir).
	if !sameNullableInt(existing.SessionInstanceID, day.SessionInstanceID) || !day.IsPresencial {
		updates["presencial_opened_at"] = nil
		updates["presencial_closed_at"] = nil
	}
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func sameNullableInt(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// UpsertMany (design.md D5): 1 INSERT multi-fila con ON CONFLICT y RETURNING.
// La limpieza presencial Gap-26 va en el DO UPDATE; `t` es la fila actual
// (heap tuple) y `EXCLUDED` la propuesta de inserción — el CASE conserva
// opened/closed salvo que la escritura cambie la instancia o apague
// is_presencial.
func (d *groupCalendarDayDao) UpsertMany(ctx *gin.Context, days []*dbs.GroupCalendarDay) ([]GroupCalendarDayUpsertResult, error) {
	if len(days) == 0 {
		return nil, nil
	}
	// Dedupe por (group_id, date): filas duplicadas en el input romperían
	// Postgres con "ON CONFLICT DO UPDATE command cannot affect row a second
	// time". Se escribe una sola fila por clave, ganando la ÚLTIMA ocurrencia
	// (last-write-wins, como toleraba el loop secuencial). El slice de
	// resultado conserva len(days): cada posición de input (incluyendo
	// duplicados) se mapea a la fila física de su clave.
	dayKey := func(day *dbs.GroupCalendarDay) string {
		return fmt.Sprintf("%d|%s", day.GroupID, day.Date.Format("2006-01-02"))
	}
	lastForKey := make(map[string]*dbs.GroupCalendarDay, len(days))
	orderedKeys := make([]string, 0, len(days))
	for _, day := range days {
		key := dayKey(day)
		if _, ok := lastForKey[key]; !ok {
			orderedKeys = append(orderedKeys, key)
		}
		lastForKey[key] = day
	}
	values := make([]string, 0, len(orderedKeys))
	args := make([]interface{}, 0, len(days)*13)
	// El insert físico no tiene defaults de DB: created_at/updated_at vienen
	// de la app (igual que hace gorm en el Create de fila única).
	now := time.Now()
	for _, key := range orderedKeys {
		day := lastForKey[key]
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			day.GroupID, day.Date, day.Kind, day.OtherName, day.SessionInstanceID, day.CancelledReason,
			day.IsPresencial, day.PresencialTimeFrom, day.PresencialTimeTo, day.PresencialLocation, day.SourcePlanID,
			now, now,
		)
	}
	// El SQL tiene que empezar exactamente con "INSERT": testutils.opOfSQL
	// clasifica la operación por prefijo del texto de la query.
	insertPrefix := "INSERT INTO group_calendar_days (" +
		"group_id, date, kind, other_name, session_instance_id, " +
		"cancelled_reason, is_presencial, presencial_time_from, " +
		"presencial_time_to, presencial_location, source_plan_id, " +
		"created_at, updated_at" +
		") VALUES "
	query := insertPrefix + strings.Join(values, ", ") + `
		ON CONFLICT (group_id, date) DO UPDATE SET
			kind = EXCLUDED.kind,
			other_name = EXCLUDED.other_name,
			session_instance_id = EXCLUDED.session_instance_id,
			cancelled_reason = EXCLUDED.cancelled_reason,
			is_presencial = EXCLUDED.is_presencial,
			presencial_time_from = EXCLUDED.presencial_time_from,
			presencial_time_to = EXCLUDED.presencial_time_to,
			presencial_location = EXCLUDED.presencial_location,
			source_plan_id = EXCLUDED.source_plan_id,
			updated_at = NOW(),
			presencial_opened_at = CASE
				WHEN (EXCLUDED.session_instance_id IS DISTINCT FROM group_calendar_days.session_instance_id)
					OR (NOT EXCLUDED.is_presencial)
				THEN NULL
				ELSE group_calendar_days.presencial_opened_at
			END,
			presencial_closed_at = CASE
				WHEN (EXCLUDED.session_instance_id IS DISTINCT FROM group_calendar_days.session_instance_id)
					OR (NOT EXCLUDED.is_presencial)
				THEN NULL
				ELSE group_calendar_days.presencial_closed_at
			END
	RETURNING id, group_id, TO_CHAR(date, 'YYYY-MM-DD') AS date_key, created_at, updated_at
	`
	var returned []struct {
		ID      int64     `gorm:"column:id"`
		GroupID int64     `gorm:"column:group_id"`
		DateKey string    `gorm:"column:date_key"`
		Created time.Time `gorm:"column:created_at"`
		Updated time.Time `gorm:"column:updated_at"`
	}
	if err := d.DB.Raw(query, args...).Scan(&returned).Error; err != nil {
		return nil, fmt.Errorf("error upserting calendar days batch: %w", err)
	}
	resultsByKey := make(map[string]GroupCalendarDayUpsertResult, len(returned))
	for i := range returned {
		row := &returned[i]
		key := fmt.Sprintf("%d|%s", row.GroupID, row.DateKey)
		if _, ok := lastForKey[key]; !ok {
			return nil, fmt.Errorf("error upserting calendar days batch: RETURNING %d|%s fuera del lote", row.GroupID, row.DateKey)
		}
		resultsByKey[key] = GroupCalendarDayUpsertResult{ID: row.ID, CreatedAt: row.Created, UpdatedAt: row.Updated}
	}
	results := make([]GroupCalendarDayUpsertResult, len(days))
	for i, day := range days {
		results[i] = resultsByKey[dayKey(day)]
	}
	return results, nil
}

func (d *groupCalendarDayDao) FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
	var day dbs.GroupCalendarDay
	err := d.DB.Where("group_id = ? AND date = ?", groupID, date).First(&day).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding calendar day: %w", err)
	}
	return &day, nil
}

func (d *groupCalendarDayDao) FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
	var days []dbs.GroupCalendarDay
	err := d.DB.Where("group_id = ? AND date >= ? AND date <= ?", groupID, from, to).Order("date").Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error listing calendar days: %w", err)
	}
	return days, nil
}

// FindForGroupsInRange es el espejo multi-grupo de FindByGroupAndRange
// (design.md D8): un solo query con group_id IN en vez de N queries por
// grupo. Sin grupos devuelve vacío (evita IN () inválido).
func (d *groupCalendarDayDao) FindForGroupsInRange(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var days []dbs.GroupCalendarDay
	err := d.DB.Where("group_id IN ? AND date >= ? AND date <= ?", groupIDs, from, to).Order("date").Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error listing calendar days: %w", err)
	}
	return days, nil
}

func (d *groupCalendarDayDao) Delete(ctx *gin.Context, groupID int64, date time.Time) error {
	return d.DB.Where("group_id = ? AND date = ?", groupID, date).Delete(&dbs.GroupCalendarDay{}).Error
}

func (d *groupCalendarDayDao) FindPresencialForGroupsInRange(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 || len(dates) == 0 {
		return nil, nil
	}
	var days []dbs.GroupCalendarDay
	err := d.DB.
		Where("group_id IN ? AND date IN ? AND kind = ? AND is_presencial = ?",
			groupIDs, dates, string(constants.GroupCalendarDayKindTraining), true).
		Order("date, presencial_time_from").
		Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error finding presencial days: %w", err)
	}
	return days, nil
}

func (d *groupCalendarDayDao) FindByGroupAndDates(ctx *gin.Context, groupID int64, dates []time.Time) ([]dbs.GroupCalendarDay, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	var days []dbs.GroupCalendarDay
	err := d.DB.Where("group_id = ? AND date IN ?", groupID, dates).Order("date").Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error finding calendar days: %w", err)
	}
	return days, nil
}

func (d *groupCalendarDayDao) DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error {
	if len(dates) == 0 {
		return nil
	}
	return d.DB.Where("group_id = ? AND date IN ?", groupID, dates).Delete(&dbs.GroupCalendarDay{}).Error
}

// FindNextForGroupsByKind implementa el filtro "hoy cuenta" del banner de
// próxima sesión (design.md D6): un día de HOY solo cuenta si no es
// presencial, o si su presencial_time_from todavía no pasó — el mismo
// criterio que isCalendarDayClosed (calendar_service.go), que compara el
// hour/minute del valor en UTC contra la hora local de now. La query hace el
// mismo par de valores en SQL: TO_CHAR(... AT TIME ZONE 'UTC') para el horario
// persistido (convención UTC, ver dbs.GroupCalendarDay) contra nowHHMM, la
// hora de pared local formateada "HH:MM" por el service.
func (d *groupCalendarDayDao) FindNextForGroupsByKind(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var day dbs.GroupCalendarDay
	err := d.DB.
		Where("group_id IN ? AND kind = ?", groupIDs, kind).
		Where("date > ? OR (date = ? AND (is_presencial = ? OR presencial_time_from IS NULL OR TO_CHAR(presencial_time_from AT TIME ZONE 'UTC', 'HH24:MI') > ?))",
			today, today, false, nowHHMM).
		Order("date ASC").First(&day).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding next day by kind: %w", err)
	}
	return &day, nil
}

// FindNextPresencialForGroups es la variante training+presencial del filtro
// "hoy cuenta" (design.md D7, banner del entrenador): mismo criterio SQL que
// FindNextForGroupsByKind (espejo de isCalendarDayClosed en calendar_service.go),
// acotado a días training+presencial.
func (d *groupCalendarDayDao) FindNextPresencialForGroups(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var day dbs.GroupCalendarDay
	err := d.DB.
		Where("group_id IN ? AND kind = ? AND is_presencial = ?", groupIDs, string(constants.GroupCalendarDayKindTraining), true).
		Where("date > ? OR (date = ? AND presencial_time_from IS NOT NULL AND TO_CHAR(presencial_time_from AT TIME ZONE 'UTC', 'HH24:MI') > ?)",
			today, today, nowHHMM).
		Order("date ASC, presencial_time_from ASC, id ASC").First(&day).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding next presencial day: %w", err)
	}
	return &day, nil
}

func (d *groupCalendarDayDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("source_plan_id = ?", planID).Update("source_plan_id", nil).Error
}

func (d *groupCalendarDayDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", groupID, oldDate).Update("date", newDate).Error
}

// FindBySessionInstanceID resuelve el día que referencia la instancia (D6).
// Con 1:1 por diseño First() alcanza; si una instancia quedara referenciada
// por más de un día (Estado inconsistente fuera del diseño) devuelve el
// primero por id.
func (d *groupCalendarDayDao) FindBySessionInstanceID(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error) {
	var day dbs.GroupCalendarDay
	err := d.DB.Where("session_instance_id = ?", sessionInstanceID).Order("id").First(&day).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding calendar day by session instance: %w", err)
	}
	return &day, nil
}

// SetPresencialOpenedAt marca la apertura presencial: la query solo impacta
// si presencial_opened_at está NULL (idempotente a nivel SQL). true = esta
// llamada hizo la apertura.
func (d *groupCalendarDayDao) SetPresencialOpenedAt(ctx *gin.Context, dayID int64, at time.Time) (bool, error) {
	res := d.DB.Model(&dbs.GroupCalendarDay{}).
		Where("id = ? AND presencial_opened_at IS NULL", dayID).
		Updates(map[string]interface{}{"presencial_opened_at": at})
	if res.Error != nil {
		return false, fmt.Errorf("error setting presencial opened at: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// SetPresencialClosedAt marca el cierre presencial: guard sobre
// presencial_closed_at NULL (el cierre es final, D5). true = esta llamada
// hizo el cierre.
func (d *groupCalendarDayDao) SetPresencialClosedAt(ctx *gin.Context, dayID int64, at time.Time) (bool, error) {
	res := d.DB.Model(&dbs.GroupCalendarDay{}).
		Where("id = ? AND presencial_closed_at IS NULL", dayID).
		Updates(map[string]interface{}{"presencial_closed_at": at})
	if res.Error != nil {
		return false, fmt.Errorf("error setting presencial closed at: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}
