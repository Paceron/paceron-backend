package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type GroupCalendarDaoInterface interface {
	Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	Delete(ctx *gin.Context, groupID int64, date time.Time) error
	DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error
	FindNextSessionForGroups(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error)
	FindDistinctGroupsBySession(ctx *gin.Context, sessionID int64) ([]int64, error)
	ClearSourcePlan(ctx *gin.Context, planID int64) error
	RepointSessionForGroups(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error
	UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
	FindBySessionID(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error)
	RepointDaysByID(ctx *gin.Context, dayIDs []int64, newSessionID int64) error
	FindByExerciseID(ctx *gin.Context, exerciseID int64) ([]dbs.GroupCalendarDay, error)
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

func (d *groupCalendarDayDao) DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error {
	if len(dates) == 0 {
		return nil
	}
	return d.DB.Where("group_id = ? AND date IN ?", groupID, dates).Delete(&dbs.GroupCalendarDay{}).Error
}

func (d *groupCalendarDayDao) FindNextSessionForGroups(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var day dbs.GroupCalendarDay
	err := d.DB.Where("group_id IN ? AND kind IN ? AND date >= ?", groupIDs, []string{"training", "cancelled"}, fromDate).
		Order("date ASC").First(&day).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding next session: %w", err)
	}
	return &day, nil
}

func (d *groupCalendarDayDao) FindDistinctGroupsBySession(ctx *gin.Context, sessionID int64) ([]int64, error) {
	var groupIDs []int64
	err := d.DB.Model(&dbs.GroupCalendarDay{}).Where("session_instance_id = ?", sessionID).Distinct().Pluck("group_id", &groupIDs).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups by session: %w", err)
	}
	return groupIDs, nil
}

func (d *groupCalendarDayDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("source_plan_id = ?", planID).Update("source_plan_id", nil).Error
}

func (d *groupCalendarDayDao) RepointSessionForGroups(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	return d.DB.Model(&dbs.GroupCalendarDay{}).
		Where("group_id IN ? AND session_instance_id = ?", groupIDs, oldSessionID).
		Update("session_instance_id", newSessionID).Error
}

func (d *groupCalendarDayDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", groupID, oldDate).Update("date", newDate).Error
}

func (d *groupCalendarDayDao) FindBySessionID(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error) {
	var days []dbs.GroupCalendarDay
	err := d.DB.Where("session_instance_id = ?", sessionID).Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error finding calendar days by session: %w", err)
	}
	return days, nil
}

func (d *groupCalendarDayDao) RepointDaysByID(ctx *gin.Context, dayIDs []int64, newSessionID int64) error {
	if len(dayIDs) == 0 {
		return nil
	}
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("id IN ?", dayIDs).Update("session_instance_id", newSessionID).Error
}

// FindByExerciseID devuelve todos los días de calendario cuya sesión asignada
// referencia el ejercicio dado (join por session_exercises.session_id) —
// usado para congelar sesiones con días cerrados cuando se edita un Exercise
// directamente, no solo cuando se edita la Session (ver design.md D5 de
// congelar-ejercicio-en-clon).
func (d *groupCalendarDayDao) FindByExerciseID(ctx *gin.Context, exerciseID int64) ([]dbs.GroupCalendarDay, error) {
	var days []dbs.GroupCalendarDay
	err := d.DB.Table("group_calendar_days").
		Select("group_calendar_days.*").
		Joins("JOIN session_exercises ON session_exercises.session_id = group_calendar_days.session_instance_id").
		Where("session_exercises.exercise_id = ?", exerciseID).
		Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error finding calendar days by exercise: %w", err)
	}
	return days, nil
}
