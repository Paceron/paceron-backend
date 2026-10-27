package main

import (
	"fmt"
	"time"
)

// Geometría de la ventana del seed. El plan tiene 2 semanas de 7 días (tarea
// 7.5) y el stamp mapea sequence_no 1..7 a los 7 días consecutivos desde
// start_date: el martes es el sequence_no 2 y el jueves el 4.
const (
	planDaysPerWeek   = 7
	planDayTuesday    = 2
	planDayThursday   = 4
	seedWeeks         = 2
	seedSessionsCount = seedWeeks * 2 // 2 por semana: martes y jueves
)

const dateLayout = "2006-01-02"

// seedWindow son las dos semanas completas que el seed siembra: arranca en el
// lunes de la primera y termina en el domingo de la segunda.
type seedWindow struct {
	Start time.Time // lunes de la primera semana, 00:00 UTC
}

// defaultWindow devuelve las dos semanas completas que terminan la semana pasada.
// Es el default porque el dashboard muestra `date_range` y un seed cuya última
// sesión es de hace un mes se ve como un equipo abandonado; dos semanas atrás
// deja el rango pegado a "hoy" y aun así exige SEEDING_ENABLED para estampar
// (el stamp del día en curso no lo abre la bandera, a propósito).
func defaultWindow(today time.Time) seedWindow {
	currentWeekMonday := startOfWeek(today)
	return seedWindow{Start: currentWeekMonday.AddDate(0, 0, -7*seedWeeks)}
}

// startOfWeek devuelve el lunes de la semana de d (lunes ISO, como date_trunc
// 'week' en Postgres — el mismo anclaje que usa el bucket semanal del dashboard).
func startOfWeek(d time.Time) time.Time {
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	// time.Weekday() da domingo=0; el lunes ISO es 1.
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func (w seedWindow) weekStart(weekIndex int) time.Time {
	return w.Start.AddDate(0, 0, weekIndex*planDaysPerWeek)
}

func (w seedWindow) from() time.Time { return w.Start }

func (w seedWindow) to() time.Time { return w.weekStart(seedWeeks-1).AddDate(0, 0, planDaysPerWeek-1) }

// dayDate es la fecha de la sesión #dayIndex del seed, en orden cronológico.
func (w seedWindow) dayDate(dayIndex int) time.Time {
	return w.Start.AddDate(0, 0, dayIndex)
}

// weekIndexOfDay mapea el índice de día al índice de semana (0 o 1), que es lo
// que usa el generador para la progresión semanal.
func (w seedWindow) weekIndexOfDay(dayIndex int) int {
	return dayIndex / planDaysPerWeek
}

// stampTargetDates son las fechas que se estamparan: martes y jueves de cada
// semana, en orden.
func (w seedWindow) stampTargetDates() []time.Time {
	dates := make([]time.Time, 0, seedSessionsCount)
	for day := 0; day < seedWeeks*planDaysPerWeek; day++ {
		if day%planDaysPerWeek+1 == planDayTuesday || day%planDaysPerWeek+1 == planDayThursday {
			dates = append(dates, w.dayDate(day))
		}
	}
	return dates
}

// excludeDatesForWeek son los 5 días de la semana que NO se estamparan. Van en
// exclude_dates del stamp: el seed no crea los días de descanso, los saltea.
func (w seedWindow) excludeDatesForWeek(weekIndex int) []string {
	weekStart := w.weekStart(weekIndex)
	excluded := make([]string, 0, planDaysPerWeek-2)
	for seq := 1; seq <= planDaysPerWeek; seq++ {
		if seq == planDayTuesday || seq == planDayThursday {
			continue
		}
		excluded = append(excluded, weekStart.AddDate(0, 0, seq-1).Format(dateLayout))
	}
	return excluded
}

func (w seedWindow) String() string {
	return fmt.Sprintf("%s..%s (%d sesiones: martes y jueves de %d semanas)",
		w.from().Format(dateLayout), w.to().Format(dateLayout), seedSessionsCount, seedWeeks)
}
