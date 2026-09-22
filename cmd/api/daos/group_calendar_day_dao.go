package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type GroupCalendarDaoInterface interface {
	Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	// FindPresencialForGroupsInRange devuelve los días training+presencial de
	// los grupos indicados en las fechas indicadas (base de la detección de
	// colisiones presenciales, design.md D3).
	FindPresencialForGroupsInRange(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)
	Delete(ctx *gin.Context, groupID int64, date time.Time) error
	DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error
	// FindNextForGroupsByKind devuelve el día más próximo del kind indicado
	// entre los grupos dados con el filtro "hoy cuenta" del banner (design.md
	// D6): date > hoy, o date == hoy solo si no es presencial o el horario
	// presencial todavía no arrancó (mismo criterio que isCalendarDayClosed).
	// nowHHMM es la hora actual en "HH:MM"; today es la medianoche local.
	FindNextForGroupsByKind(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error)
	ClearSourcePlan(ctx *gin.Context, planID int64) error
	UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
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
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
		"kind":                 day.Kind,
		"other_name":           day.OtherName,
		"session_instance_id":  day.SessionInstanceID,
		"cancelled_reason":     day.CancelledReason,
		"is_presencial":        day.IsPresencial,
		"presencial_time_from": day.PresencialTimeFrom,
		"presencial_time_to":   day.PresencialTimeTo,
		"presencial_location":  day.PresencialLocation,
		"source_plan_id":       day.SourcePlanID,
	}).Error
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

func (d *groupCalendarDayDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("source_plan_id = ?", planID).Update("source_plan_id", nil).Error
}

func (d *groupCalendarDayDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", groupID, oldDate).Update("date", newDate).Error
}
