package dbs

import "time"

// PlatformSetting es una tabla genérica key-value para configuración global de
// la aplicación. La clave inicial es `marketplace_fee_percent` (comisión que
// Paceron retiene del split de cuotas de equipo). `value` es JSONB para poder
// guardar cualquier tipo. updated_by registra quién la modificó.
type PlatformSetting struct {
	Key       string     `gorm:"column:key;primaryKey"`
	Value     string     `gorm:"column:value;type:jsonb;not null"` // JSON serializado
	UpdatedBy *int64     `gorm:"column:updated_by"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (PlatformSetting) TableName() string {
	return "platform_settings"
}