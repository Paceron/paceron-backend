package dbs

import "time"

// GroupCalendarDay es una fila dispersa del calendario real de un grupo —
// sin fila para una fecha significa día vacío. UNIQUE(group_id, date) se
// aplica vía índice compuesto (Step siguiente), no acá.
type GroupCalendarDay struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	GroupID   int64     `gorm:"column:group_id;not null;uniqueIndex:idx_group_calendar_day_group_date"`
	Date      time.Time `gorm:"column:date;type:date;not null;uniqueIndex:idx_group_calendar_day_group_date"`
	Kind      string    `gorm:"column:kind;not null"`
	OtherName *string   `gorm:"column:other_name"`
	// SessionInstanceID apunta a una copia inmutable de la Session del catálogo
	// (design.md asignacion-por-instanciacion D1) — nunca al catálogo.
	SessionInstanceID *int64  `gorm:"column:session_instance_id"`
	CancelledReason   *string `gorm:"column:cancelled_reason"`
	IsPresencial      bool    `gorm:"column:is_presencial;not null;default:false"`
	// PresencialTimeFrom/PresencialTimeTo son horarios sueltos (sin fecha real)
	// persistidos como timestamptz con fecha 0000-01-01 — Postgres siempre
	// devuelve timestamptz convertido al TimeZone de la sesión, y el driver lo
	// escanea en time.Local, así que estos valores SIEMPRE se escriben y leen
	// en UTC por convención (nunca confiar en .Location() del valor leído,
	// llamar .UTC() antes de leer Hour()/Minute()) — si no, la hora se corrompe
	// por el offset de time.Local en cada round-trip.
	PresencialTimeFrom *time.Time `gorm:"column:presencial_time_from"`
	PresencialTimeTo   *time.Time `gorm:"column:presencial_time_to"`
	PresencialLocation *string    `gorm:"column:presencial_location;type:jsonb"`
	SourcePlanID       *int64     `gorm:"column:source_plan_id"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (GroupCalendarDay) TableName() string { return "group_calendar_days" }
