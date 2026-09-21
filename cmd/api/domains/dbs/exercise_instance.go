package dbs

import "time"

// ExerciseInstance es la copia inmutable de un catálogo Exercise, creada al
// asignar contenido a un día de calendario. Sin owner_id/deleted_at/updated_at:
// nunca se edita ni se lista como catálogo (design.md D1).
type ExerciseInstance struct {
	ID          int64    `gorm:"column:id;primaryKey"`
	Name        string   `gorm:"column:name;not null"`
	Description *string  `gorm:"column:description"`
	Kind        string   `gorm:"column:kind;not null"`
	Intensity   *string  `gorm:"column:intensity"`
	Minutes     *int     `gorm:"column:minutes"`
	DistanceM   *int     `gorm:"column:distance_m"`
	SpeedKph    *float64 `gorm:"column:speed_kph;type:numeric(4,1)"`
	MuscleGroup *string  `gorm:"column:muscle_group"`
	VideoURL    *string  `gorm:"column:video_url"`
	// SourceExerciseID es una referencia informativa (opaca, app-managed, sin
	// FK de DB) al Exercise del catálogo del que se instanció; NULL en
	// instancias anteriores al change instancia-referencia-catalogo.
	SourceExerciseID *int64    `gorm:"column:source_exercise_id"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ExerciseInstance) TableName() string { return "exercise_instances" }
