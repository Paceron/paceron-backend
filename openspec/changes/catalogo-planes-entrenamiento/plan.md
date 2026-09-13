# Catálogo de planes de entrenamiento — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar el catálogo reusable de Exercise/Session/TrainingPlan (CRUD + clone), cerrando la mitad "catálogo" del Gap 4 de `paceron-frontend/docs/BACKEND_API_GAPS.md`.

**Architecture:** Controllers → Services → DAOs, sin delegate (cada service resuelve su propia composición de DAOs, incluso cruzando a DAOs de otra entidad de este mismo change — ej. `SessionService` valida `exercise_id` contra `ExerciseDaoInterface` directo). Error shape propio `{"message":"..."}` (no `apierror.APIError`), pedido explícito de la spec de frontend.

**Tech Stack:** Go 1.26, Gin, GORM/PostgreSQL, testify, mismo molde de mocks del repo (`mockXxxDao{fnField: func(...){...}}`).

**Spec:** `openspec/changes/catalogo-planes-entrenamiento/{proposal.md,design.md,tasks.md,specs/}` + `paceron-frontend/docs/BACKEND_TRAINING_PLANS_SPEC.md` (fuente original).

## Global Constraints

- Comunicación/comentarios/mensajes de error en español.
- Error de este dominio SIEMPRE `{"message": "..."}` + status semántico — nunca `apierror.APIError` (D6).
- Enums como `string` + paquete `constants`, nunca enum nativo de Postgres (D2).
- Soft-delete de `Exercise`/`Session` vía `deleted_at`; `TrainingPlan` se borra físico (D1).
- `PUT` de `Session`/`TrainingPlan` con `days`/`exercises` reemplaza el set hijo entero, nunca patch fila por fila.
- **Autorización por dueño** (D9): `Create` exige `req.OwnerID == callerID`; `Update`/`Delete`/`Clone` exigen `existing.OwnerID == callerID`. `ErrCatalogForbidden` → `403`. `callerID` sale de `utils.GetAuthUserID(c)` en el controller, se pasa a cada método de service.
- `go build ./...` y `go test ./...` (sin DB) deben quedar verdes después de cada tarea. Tests de DAO corren contra Postgres real (`testutils.SetupTestDB(t)`) y se skipean solos sin `TEST_DB_HOST` — no bloquean tareas intermedias, pero task 18 los corre con DB antes de dar el change por cerrado.
- No usar delegates. No agregar paginación (spec explícita: sin paginación en catálogo).

---

### Task 1: Modelos DB + constants + migración

**Files:**
- Create: `cmd/api/domains/dbs/exercise.go`
- Create: `cmd/api/domains/dbs/session.go`
- Create: `cmd/api/domains/dbs/session_exercise.go`
- Create: `cmd/api/domains/dbs/training_plan.go`
- Create: `cmd/api/domains/dbs/plan_day.go`
- Create: `cmd/api/domains/constants/exercise_kind.go`
- Create: `cmd/api/domains/constants/exercise_intensity.go`
- Create: `cmd/api/domains/constants/muscle_group.go`
- Create: `cmd/api/domains/constants/session_exercise_role.go`
- Create: `cmd/api/domains/constants/plan_day_kind.go`
- Modify: `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate)
- Test: `cmd/api/domains/constants/exercise_kind_test.go` (y análogos para los otros 4)

**Interfaces:**
- Produces: `dbs.Exercise`, `dbs.Session`, `dbs.SessionExercise`, `dbs.TrainingPlan`, `dbs.PlanDay` (campos exactos abajo); `constants.ExerciseKind`/`GetValidExerciseKinds()`/`IsValidExerciseKind(string) bool` (y los 4 análogos por cada enum).

- [ ] **Step 1: Crear los 5 modelos DB**

`cmd/api/domains/dbs/exercise.go`:
```go
package dbs

import "time"

// Exercise es un ítem del catálogo reusable de un entrenador. kind/intensity/
// muscle_group son independientes entre sí — sin combinación obligatoria.
type Exercise struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	OwnerID     int64      `gorm:"column:owner_id;not null"`
	Name        string     `gorm:"column:name;not null"`
	Description *string    `gorm:"column:description"`
	Kind        string     `gorm:"column:kind;not null"`
	Intensity   *string    `gorm:"column:intensity"`
	Minutes     *int       `gorm:"column:minutes"`
	DistanceM   *int       `gorm:"column:distance_m"`
	SpeedKph    *float64   `gorm:"column:speed_kph;type:numeric(4,1)"`
	MuscleGroup *string    `gorm:"column:muscle_group"`
	VideoURL    *string    `gorm:"column:video_url"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Exercise) TableName() string { return "exercises" }
```

`cmd/api/domains/dbs/session.go`:
```go
package dbs

import "time"

// Session es una plantilla de entrenamiento reusable, compuesta por N
// SessionExercise (tabla propia, no array embebido).
type Session struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	OwnerID     int64      `gorm:"column:owner_id;not null"`
	Name        string     `gorm:"column:name;not null"`
	Description *string    `gorm:"column:description"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Session) TableName() string { return "sessions" }
```

`cmd/api/domains/dbs/session_exercise.go`:
```go
package dbs

// SessionExercise vincula una Session con un Exercise y su rol dentro de la
// sesión. Sin campo de orden explícito — el ID autoincremental desempata.
type SessionExercise struct {
	ID          int64 `gorm:"column:id;primaryKey"`
	SessionID   int64 `gorm:"column:session_id;not null"`
	ExerciseID  int64 `gorm:"column:exercise_id;not null"`
	Role        string `gorm:"column:role;not null"`
	RepeatCount int   `gorm:"column:repeat_count;not null;default:1"`
	RestMinutes int   `gorm:"column:rest_minutes;not null;default:0"`
}

func (SessionExercise) TableName() string { return "session_exercises" }
```

`cmd/api/domains/dbs/training_plan.go`:
```go
package dbs

import "time"

// TrainingPlan es un template reusable, sin caducidad propia. Se borra
// físico (no soft-delete) — ver design.md D1.
type TrainingPlan struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	OwnerID     int64     `gorm:"column:owner_id;not null"`
	Name        string    `gorm:"column:name;not null"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrainingPlan) TableName() string { return "training_plans" }
```

`cmd/api/domains/dbs/plan_day.go`:
```go
package dbs

import "time"

// PlanDay es un día secuencial (1..N) de un TrainingPlan. Los campos
// default_* son informativos para el momento del stamp (ver el change de
// calendario), no afectan nada del catálogo en sí.
type PlanDay struct {
	ID                int64      `gorm:"column:id;primaryKey"`
	PlanID            int64      `gorm:"column:plan_id;not null"`
	SequenceNo        int        `gorm:"column:sequence_no;not null"`
	Kind              string     `gorm:"column:kind;not null"`
	OtherName         *string    `gorm:"column:other_name"`
	SessionID         *int64     `gorm:"column:session_id"`
	DefaultPresencial bool       `gorm:"column:default_presencial;not null;default:false"`
	DefaultTime       *time.Time `gorm:"column:default_time;type:time"`
	DefaultLocation   *string    `gorm:"column:default_location;type:jsonb"`
}

func (PlanDay) TableName() string { return "plan_days" }
```

- [ ] **Step 2: Crear los 5 archivos de constants**

`cmd/api/domains/constants/exercise_kind.go`:
```go
package constants

type ExerciseKind string

const (
	ExerciseKindWalking    ExerciseKind = "walking"
	ExerciseKindJogging    ExerciseKind = "jogging"
	ExerciseKindElongation ExerciseKind = "elongation"
	ExerciseKindCruising   ExerciseKind = "cruising"
	ExerciseKindRunning    ExerciseKind = "running"
)

func GetValidExerciseKinds() []string {
	return []string{
		string(ExerciseKindWalking),
		string(ExerciseKindJogging),
		string(ExerciseKindElongation),
		string(ExerciseKindCruising),
		string(ExerciseKindRunning),
	}
}

func IsValidExerciseKind(kind string) bool {
	for _, k := range GetValidExerciseKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
```

`cmd/api/domains/constants/exercise_intensity.go`:
```go
package constants

type ExerciseIntensity string

const (
	ExerciseIntensityLight    ExerciseIntensity = "light"
	ExerciseIntensityModerate ExerciseIntensity = "moderate"
	ExerciseIntensityVigorous ExerciseIntensity = "vigorous"
)

func GetValidExerciseIntensities() []string {
	return []string{
		string(ExerciseIntensityLight),
		string(ExerciseIntensityModerate),
		string(ExerciseIntensityVigorous),
	}
}

func IsValidExerciseIntensity(intensity string) bool {
	for _, i := range GetValidExerciseIntensities() {
		if i == intensity {
			return true
		}
	}
	return false
}
```

`cmd/api/domains/constants/muscle_group.go`:
```go
package constants

type MuscleGroup string

const (
	MuscleGroupCuadriceps     MuscleGroup = "cuadriceps"
	MuscleGroupIsquiotibiales MuscleGroup = "isquiotibiales"
	MuscleGroupGemelos        MuscleGroup = "gemelos"
	MuscleGroupGluteos        MuscleGroup = "gluteos"
	MuscleGroupAductores      MuscleGroup = "aductores"
	MuscleGroupPsoas          MuscleGroup = "psoas"
	MuscleGroupLumbares       MuscleGroup = "lumbares"
	MuscleGroupCore           MuscleGroup = "core"
)

func GetValidMuscleGroups() []string {
	return []string{
		string(MuscleGroupCuadriceps),
		string(MuscleGroupIsquiotibiales),
		string(MuscleGroupGemelos),
		string(MuscleGroupGluteos),
		string(MuscleGroupAductores),
		string(MuscleGroupPsoas),
		string(MuscleGroupLumbares),
		string(MuscleGroupCore),
	}
}

func IsValidMuscleGroup(group string) bool {
	for _, g := range GetValidMuscleGroups() {
		if g == group {
			return true
		}
	}
	return false
}
```

`cmd/api/domains/constants/session_exercise_role.go`:
```go
package constants

type SessionExerciseRole string

const (
	SessionExerciseRoleWarmup   SessionExerciseRole = "warmup"
	SessionExerciseRoleMain     SessionExerciseRole = "main"
	SessionExerciseRoleCooldown SessionExerciseRole = "cooldown"
)

func GetValidSessionExerciseRoles() []string {
	return []string{
		string(SessionExerciseRoleWarmup),
		string(SessionExerciseRoleMain),
		string(SessionExerciseRoleCooldown),
	}
}

func IsValidSessionExerciseRole(role string) bool {
	for _, r := range GetValidSessionExerciseRoles() {
		if r == role {
			return true
		}
	}
	return false
}
```

`cmd/api/domains/constants/plan_day_kind.go`:
```go
package constants

type PlanDayKind string

const (
	PlanDayKindRest     PlanDayKind = "rest"
	PlanDayKindOther    PlanDayKind = "other"
	PlanDayKindTraining PlanDayKind = "training"
)

func GetValidPlanDayKinds() []string {
	return []string{
		string(PlanDayKindRest),
		string(PlanDayKindOther),
		string(PlanDayKindTraining),
	}
}

func IsValidPlanDayKind(kind string) bool {
	for _, k := range GetValidPlanDayKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Test rápido de cada constants (tabla de casos)**

`cmd/api/domains/constants/exercise_kind_test.go` (mismo patrón para los otros 4 — `exercise_intensity_test.go`, `muscle_group_test.go`, `session_exercise_role_test.go`, `plan_day_kind_test.go`, ajustando el nombre de la función y los valores válidos/inválidos):
```go
package constants

import "testing"

func TestIsValidExerciseKind(t *testing.T) {
	if !IsValidExerciseKind("running") {
		t.Error("running debería ser válido")
	}
	if IsValidExerciseKind("flying") {
		t.Error("flying no debería ser válido")
	}
}

func TestGetValidExerciseKinds(t *testing.T) {
	kinds := GetValidExerciseKinds()
	if len(kinds) != 5 {
		t.Errorf("esperaba 5 kinds, obtuve %d", len(kinds))
	}
}
```

- [ ] **Step 4: Registrar AutoMigrate**

En `cmd/api/infrastructure/postgresdb/postgres.go`, agregar a la lista de `AutoMigrate(...)` (buscar la llamada existente y sumar al final de los argumentos, mismo estilo que `&dbs.JoinRequest{}` ya agregado en un change anterior):
```go
&dbs.Exercise{},
&dbs.Session{},
&dbs.SessionExercise{},
&dbs.TrainingPlan{},
&dbs.PlanDay{},
```

- [ ] **Step 5: Verificar**

Run: `go build ./... && go vet ./... && go test ./cmd/api/domains/constants/...`
Expected: build limpio, tests de constants en verde.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/domains/dbs/exercise.go cmd/api/domains/dbs/session.go cmd/api/domains/dbs/session_exercise.go cmd/api/domains/dbs/training_plan.go cmd/api/domains/dbs/plan_day.go cmd/api/domains/constants/exercise_kind.go cmd/api/domains/constants/exercise_kind_test.go cmd/api/domains/constants/exercise_intensity.go cmd/api/domains/constants/exercise_intensity_test.go cmd/api/domains/constants/muscle_group.go cmd/api/domains/constants/muscle_group_test.go cmd/api/domains/constants/session_exercise_role.go cmd/api/domains/constants/session_exercise_role_test.go cmd/api/domains/constants/plan_day_kind.go cmd/api/domains/constants/plan_day_kind_test.go cmd/api/infrastructure/postgresdb/postgres.go
git commit -m "feat(catalog): add Exercise/Session/TrainingPlan DB models and enums"
```

---

### Task 2: DTOs (request/response) de las 3 entidades + Location compartido

**Files:**
- Create: `cmd/api/domains/trainingplan/location.go`
- Create: `cmd/api/domains/exercise/exercise_request.go`
- Create: `cmd/api/domains/exercise/exercise_response.go`
- Create: `cmd/api/domains/session/session_request.go`
- Create: `cmd/api/domains/session/session_response.go`
- Create: `cmd/api/domains/trainingplan/training_plan_request.go`
- Create: `cmd/api/domains/trainingplan/training_plan_response.go`

**Interfaces:**
- Consumes: nada de tasks anteriores (DTOs puros).
- Produces: `trainingplan.Location{Lat,Lng,Label}`; `exercise.ExerciseRequest`/`ExerciseResponse`; `session.SessionExerciseRequest`/`SessionRequest`/`SessionExerciseResponse`/`SessionResponse`; `trainingplan.PlanDayRequest`/`TrainingPlanRequest`/`TrainingPlanUpdateRequest`/`PlanDayResponse`/`TrainingPlanResponse`. Estos nombres y campos exactos los consumen las tasks 4-6 (servicios) y 7-9 (DAOs no, solo servicios/controllers).

- [ ] **Step 1: `Location` compartido**

`cmd/api/domains/trainingplan/location.go`:
```go
package trainingplan

// Location es el shape {lat,lng,label?} compartido entre PlanDay.DefaultLocation
// (este paquete) y GroupCalendarDay.PresencialLocation (change de calendario).
type Location struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label *string `json:"label,omitempty"`
}
```

- [ ] **Step 2: DTOs de Exercise**

`cmd/api/domains/exercise/exercise_request.go`:
```go
package exercise

// ExerciseRequest es el body de POST/PUT /exercises — mismo shape en ambos,
// el PUT es reemplazo completo.
type ExerciseRequest struct {
	OwnerID     int64    `json:"owner_id" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description *string  `json:"description"`
	Kind        string   `json:"kind" binding:"required"`
	Intensity   *string  `json:"intensity"`
	Minutes     *int     `json:"minutes"`
	DistanceM   *int     `json:"distance_m"`
	SpeedKph    *float64 `json:"speed_kph"`
	MuscleGroup *string  `json:"muscle_group"`
}
```

`cmd/api/domains/exercise/exercise_response.go`:
```go
package exercise

import "time"

type ExerciseResponse struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Kind        string    `json:"kind"`
	Intensity   *string   `json:"intensity"`
	Minutes     *int      `json:"minutes"`
	DistanceM   *int      `json:"distance_m"`
	SpeedKph    *float64  `json:"speed_kph"`
	MuscleGroup *string   `json:"muscle_group"`
	VideoURL    *string   `json:"video_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

- [ ] **Step 3: DTOs de Session**

`cmd/api/domains/session/session_request.go`:
```go
package session

type SessionExerciseRequest struct {
	ExerciseID  int64  `json:"exercise_id" binding:"required"`
	Role        string `json:"role" binding:"required"`
	RepeatCount *int   `json:"repeat_count"`
	RestMinutes *int   `json:"rest_minutes"`
}

// SessionRequest es el body de POST/PUT /sessions — mismo shape en ambos,
// el PUT reemplaza Exercises entero. exclude_group_ids/clone_name/
// clone_description los agrega el change de calendario (no tocar acá).
type SessionRequest struct {
	OwnerID     int64                    `json:"owner_id" binding:"required"`
	Name        string                   `json:"name" binding:"required"`
	Description *string                  `json:"description"`
	Exercises   []SessionExerciseRequest `json:"exercises" binding:"required"`
}
```

`cmd/api/domains/session/session_response.go`:
```go
package session

import "time"

type SessionExerciseResponse struct {
	ID          int64  `json:"id"`
	ExerciseID  int64  `json:"exercise_id"`
	Role        string `json:"role"`
	RepeatCount int    `json:"repeat_count"`
	RestMinutes int    `json:"rest_minutes"`
}

type SessionResponse struct {
	ID          int64                      `json:"id"`
	OwnerID     int64                      `json:"owner_id"`
	Name        string                     `json:"name"`
	Description *string                    `json:"description"`
	Exercises   []SessionExerciseResponse `json:"exercises"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}
```

- [ ] **Step 4: DTOs de TrainingPlan**

`cmd/api/domains/trainingplan/training_plan_request.go`:
```go
package trainingplan

// PlanDayRequest.DefaultTime viaja como "HH:MM" (string) — el service lo
// parsea con time.Parse("15:04", ...).
type PlanDayRequest struct {
	SequenceNo        int       `json:"sequence_no" binding:"required"`
	Kind              string    `json:"kind" binding:"required"`
	OtherName         *string   `json:"other_name"`
	SessionID         *int64    `json:"session_id"`
	DefaultPresencial *bool     `json:"default_presencial"`
	DefaultTime       *string   `json:"default_time"`
	DefaultLocation   *Location `json:"default_location"`
}

// TrainingPlanRequest es el body de POST /training-plans (reemplazo total).
type TrainingPlanRequest struct {
	OwnerID     int64            `json:"owner_id" binding:"required"`
	Name        string           `json:"name" binding:"required"`
	Description *string          `json:"description"`
	Days        []PlanDayRequest `json:"days" binding:"required"`
}

// TrainingPlanUpdateRequest es el body de PUT /training-plans/{id} — parcial,
// Days (si viene) reemplaza el set entero.
type TrainingPlanUpdateRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	Days        *[]PlanDayRequest `json:"days"`
}
```

`cmd/api/domains/trainingplan/training_plan_response.go`:
```go
package trainingplan

import "time"

type PlanDayResponse struct {
	ID                int64     `json:"id"`
	SequenceNo        int       `json:"sequence_no"`
	Kind              string    `json:"kind"`
	OtherName         *string   `json:"other_name"`
	SessionID         *int64    `json:"session_id"`
	DefaultPresencial bool      `json:"default_presencial"`
	DefaultTime       *string   `json:"default_time"`
	DefaultLocation   *Location `json:"default_location"`
}

type TrainingPlanResponse struct {
	ID          int64             `json:"id"`
	OwnerID     int64             `json:"owner_id"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Days        []PlanDayResponse `json:"days"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
```

- [ ] **Step 5: Verificar y commitear**

Run: `go build ./...`
Expected: verde (son DTOs puros, sin lógica).

```bash
git add cmd/api/domains/trainingplan/location.go cmd/api/domains/exercise cmd/api/domains/session cmd/api/domains/trainingplan
git commit -m "feat(catalog): add request/response DTOs for exercise/session/training-plan"
```

---

### Task 3: DAO de Exercise

**Files:**
- Create: `cmd/api/daos/exercise_dao.go`
- Test: `cmd/api/daos/exercise_dao_test.go`

**Interfaces:**
- Consumes: `dbs.Exercise` (Task 1).
- Produces: `ExerciseDaoInterface{Create,FindByID,FindByOwner,Update,SoftDelete}` — usado por `ExerciseService` (Task 7) y `SessionService` (Task 8, para validar `exercise_id`).

- [ ] **Step 1: Escribir el test (Postgres real)**

`cmd/api/daos/exercise_dao_test.go`:
```go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestExerciseDao_ImplementsInterface(t *testing.T) {
	dao := NewExerciseDao(&gorm.DB{})
	var iface ExerciseDaoInterface = dao
	_ = iface
}

func TestExerciseDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-1@test.com", "60000001")
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "Sentadillas", Kind: "running"}

	err := dao.Create(nil, e)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Sentadillas", found.Name)
}

func TestExerciseDao_FindByID_NotFoundReturnsNilNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestExerciseDao_FindByOwner_ExcludesDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-2@test.com", "60000002")
	visible := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote suave", Kind: "jogging"}
	require.NoError(t, dao.Create(nil, visible))
	deleted := &dbs.Exercise{OwnerID: owner.ID, Name: "Viejo", Kind: "walking"}
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "Trote suave")
	assert.NotContains(t, names, "Viejo")
}

func TestExerciseDao_Update_ClearsOptionalFieldsToNull(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-3@test.com", "60000003")
	minutes := 30
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "Con minutos", Kind: "running", Minutes: &minutes}
	require.NoError(t, dao.Create(nil, e))

	e.Minutes = nil
	e.Name = "Sin minutos"
	err := dao.Update(nil, e)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "Sin minutos", found.Name)
	assert.Nil(t, found.Minutes)
}

func TestExerciseDao_SoftDelete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-4@test.com", "60000004")
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "A borrar", Kind: "running"}
	require.NoError(t, dao.Create(nil, e))

	err := dao.SoftDelete(nil, e.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test go test ./cmd/api/daos/... -run TestExerciseDao -v -count=1` (levantar antes con `make test-db-up` si no está corriendo)
Expected: FAIL — `NewExerciseDao`/`ExerciseDaoInterface` no existen todavía.

- [ ] **Step 3: Implementar el DAO**

`cmd/api/daos/exercise_dao.go`:
```go
package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type ExerciseDaoInterface interface {
	Create(ctx *gin.Context, e *dbs.Exercise) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error)
	Update(ctx *gin.Context, e *dbs.Exercise) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type exerciseDao struct {
	DB *gorm.DB
}

func NewExerciseDao(database *gorm.DB) ExerciseDaoInterface {
	return &exerciseDao{DB: database}
}

func (d *exerciseDao) Create(ctx *gin.Context, e *dbs.Exercise) error {
	return d.DB.Create(e).Error
}

func (d *exerciseDao) FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
	var e dbs.Exercise
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&e).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding exercise: %w", err)
	}
	return &e, nil
}

func (d *exerciseDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) {
	var exercises []dbs.Exercise
	err := d.DB.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("id").Find(&exercises).Error
	if err != nil {
		return nil, fmt.Errorf("error listing exercises: %w", err)
	}
	return exercises, nil
}

// Update usa un map (no Updates(struct)) para que los punteros nil sí
// limpien la columna a NULL — Updates(struct) de GORM omite campos zero-value.
func (d *exerciseDao) Update(ctx *gin.Context, e *dbs.Exercise) error {
	return d.DB.Model(&dbs.Exercise{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":         e.Name,
		"description":  e.Description,
		"kind":         e.Kind,
		"intensity":    e.Intensity,
		"minutes":      e.Minutes,
		"distance_m":   e.DistanceM,
		"speed_kph":    e.SpeedKph,
		"muscle_group": e.MuscleGroup,
	}).Error
}

func (d *exerciseDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Exercise{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS, los 5 tests en verde.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/daos/exercise_dao.go cmd/api/daos/exercise_dao_test.go
git commit -m "feat(catalog): add ExerciseDao with soft-delete"
```

---

### Task 4: DAO de Session + SessionExercise

**Files:**
- Create: `cmd/api/daos/session_dao.go`
- Create: `cmd/api/daos/session_exercise_dao.go`
- Test: `cmd/api/daos/session_dao_test.go`
- Test: `cmd/api/daos/session_exercise_dao_test.go`

**Interfaces:**
- Consumes: `dbs.Session`, `dbs.SessionExercise` (Task 1).
- Produces: `SessionDaoInterface{Create,FindByID,FindByOwner,Update,SoftDelete}` (mismo shape que `ExerciseDaoInterface`); `SessionExerciseDaoInterface{FindBySession,ReplaceForSession}` — usados por `SessionService` (Task 8).

- [ ] **Step 1: Test + implementación de `SessionDaoInterface`**

`cmd/api/daos/session_dao_test.go`:
```go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionDao(&gorm.DB{})
	var iface SessionDaoInterface = dao
	_ = iface
}

func TestSessionDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-1@test.com", "61000001")
	s := &dbs.Session{OwnerID: owner.ID, Name: "Sesión base"}

	err := dao.Create(nil, s)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, s.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Sesión base", found.Name)
}

func TestSessionDao_FindByOwner_ExcludesDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-2@test.com", "61000002")
	visible := &dbs.Session{OwnerID: owner.ID, Name: "Visible"}
	require.NoError(t, dao.Create(nil, visible))
	deleted := &dbs.Session{OwnerID: owner.ID, Name: "Borrada"}
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "Visible")
	assert.NotContains(t, names, "Borrada")
}

func TestSessionDao_SoftDelete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-3@test.com", "61000003")
	s := &dbs.Session{OwnerID: owner.ID, Name: "A borrar"}
	require.NoError(t, dao.Create(nil, s))

	err := dao.SoftDelete(nil, s.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, s.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
```

`cmd/api/daos/session_dao.go`:
```go
package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type SessionDaoInterface interface {
	Create(ctx *gin.Context, s *dbs.Session) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Session, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error)
	Update(ctx *gin.Context, s *dbs.Session) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type sessionDao struct {
	DB *gorm.DB
}

func NewSessionDao(database *gorm.DB) SessionDaoInterface {
	return &sessionDao{DB: database}
}

func (d *sessionDao) Create(ctx *gin.Context, s *dbs.Session) error {
	return d.DB.Create(s).Error
}

func (d *sessionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Session, error) {
	var s dbs.Session
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding session: %w", err)
	}
	return &s, nil
}

func (d *sessionDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
	var sessions []dbs.Session
	err := d.DB.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("id").Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error listing sessions: %w", err)
	}
	return sessions, nil
}

func (d *sessionDao) Update(ctx *gin.Context, s *dbs.Session) error {
	return d.DB.Model(&dbs.Session{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
		"name":        s.Name,
		"description": s.Description,
	}).Error
}

func (d *sessionDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Session{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}
```

- [ ] **Step 2: Test + implementación de `SessionExerciseDaoInterface`**

`cmd/api/daos/session_exercise_dao_test.go`:
```go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionExerciseDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionExerciseDao(&gorm.DB{})
	var iface SessionExerciseDaoInterface = dao
	_ = iface
}

func TestSessionExerciseDao_ReplaceForSession_FullCycle(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionDao(db)
	exerciseDao := NewExerciseDao(db)
	dao := NewSessionExerciseDao(db)
	owner := persistUser(db, "session-exercise-owner-1@test.com", "62000001")
	s := &dbs.Session{OwnerID: owner.ID, Name: "Con ejercicios"}
	require.NoError(t, sessionDao.Create(nil, s))
	warmup := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote suave", Kind: "jogging"}
	require.NoError(t, exerciseDao.Create(nil, warmup))
	main := &dbs.Exercise{OwnerID: owner.ID, Name: "Serie fuerte", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, main))

	err := dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: main.ID, Role: "main", RepeatCount: 3, RestMinutes: 2},
	})

	require.NoError(t, err)
	rows, findErr := dao.FindBySession(nil, s.ID)
	require.NoError(t, findErr)
	require.Len(t, rows, 2)
	assert.Equal(t, "warmup", rows[0].Role)
	assert.Equal(t, "main", rows[1].Role)
}

func TestSessionExerciseDao_ReplaceForSession_ReplacesEntireSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionDao(db)
	exerciseDao := NewExerciseDao(db)
	dao := NewSessionExerciseDao(db)
	owner := persistUser(db, "session-exercise-owner-2@test.com", "62000002")
	s := &dbs.Session{OwnerID: owner.ID, Name: "A reemplazar"}
	require.NoError(t, sessionDao.Create(nil, s))
	first := &dbs.Exercise{OwnerID: owner.ID, Name: "Primero", Kind: "walking"}
	require.NoError(t, exerciseDao.Create(nil, first))
	second := &dbs.Exercise{OwnerID: owner.ID, Name: "Segundo", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, second))
	require.NoError(t, dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: first.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
	}))

	err := dao.ReplaceForSession(nil, s.ID, []dbs.SessionExercise{
		{ExerciseID: second.ID, Role: "cooldown", RepeatCount: 1, RestMinutes: 0},
	})

	require.NoError(t, err)
	rows, findErr := dao.FindBySession(nil, s.ID)
	require.NoError(t, findErr)
	require.Len(t, rows, 1)
	assert.Equal(t, second.ID, rows[0].ExerciseID)
	assert.Equal(t, "cooldown", rows[0].Role)
}
```

`cmd/api/daos/session_exercise_dao.go`:
```go
package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type SessionExerciseDaoInterface interface {
	FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error)
	ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error
}

type sessionExerciseDao struct {
	DB *gorm.DB
}

func NewSessionExerciseDao(database *gorm.DB) SessionExerciseDaoInterface {
	return &sessionExerciseDao{DB: database}
}

func (d *sessionExerciseDao) FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
	var rows []dbs.SessionExercise
	err := d.DB.Where("session_id = ?", sessionID).Order("id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing session exercises: %w", err)
	}
	return rows, nil
}

// ReplaceForSession borra el set anterior y crea el nuevo en una
// transacción — el PUT de Session reemplaza el conjunto entero, nunca
// patchea fila por fila (spec §3.3).
func (d *sessionExerciseDao) ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&dbs.SessionExercise{}).Error; err != nil {
			return fmt.Errorf("error clearing session exercises: %w", err)
		}
		for i := range rows {
			rows[i].ID = 0
			rows[i].SessionID = sessionID
			if err := tx.Create(&rows[i]).Error; err != nil {
				return fmt.Errorf("error creating session exercise: %w", err)
			}
		}
		return nil
	})
}
```

- [ ] **Step 3: Correr los tests, verificar que pasan**

Run: `go test ./cmd/api/daos/... -run "TestSessionDao|TestSessionExerciseDao" -v -count=1` (con `TEST_DB_HOST` seteada)
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/daos/session_dao.go cmd/api/daos/session_dao_test.go cmd/api/daos/session_exercise_dao.go cmd/api/daos/session_exercise_dao_test.go
git commit -m "feat(catalog): add SessionDao and SessionExerciseDao"
```

---

### Task 5: DAO de TrainingPlan + PlanDay

**Files:**
- Create: `cmd/api/daos/training_plan_dao.go`
- Create: `cmd/api/daos/plan_day_dao.go`
- Test: `cmd/api/daos/training_plan_dao_test.go`
- Test: `cmd/api/daos/plan_day_dao_test.go`

**Interfaces:**
- Consumes: `dbs.TrainingPlan`, `dbs.PlanDay` (Task 1).
- Produces: `TrainingPlanDaoInterface{Create,FindByID,FindByOwner,Update,Delete}` (`Delete` es físico, borra también sus `PlanDay` en la misma transacción); `PlanDayDaoInterface{FindByPlan,ReplaceForPlan}` — usados por `TrainingPlanService` (Task 9).

- [ ] **Step 1: Test + implementación de `TrainingPlanDaoInterface`**

`cmd/api/daos/training_plan_dao_test.go`:
```go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestTrainingPlanDao_ImplementsInterface(t *testing.T) {
	dao := NewTrainingPlanDao(&gorm.DB{})
	var iface TrainingPlanDaoInterface = dao
	_ = iface
}

func TestTrainingPlanDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainingPlanDao(db)
	owner := persistUser(db, "plan-owner-1@test.com", "63000001")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan base"}

	err := dao.Create(nil, p)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, p.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Plan base", found.Name)
}

func TestTrainingPlanDao_FindByOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainingPlanDao(db)
	owner := persistUser(db, "plan-owner-2@test.com", "63000002")
	require.NoError(t, dao.Create(nil, &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan A"}))
	require.NoError(t, dao.Create(nil, &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan B"}))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestTrainingPlanDao_Delete_CascadesPlanDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dayDao := NewPlanDayDao(db)
	owner := persistUser(db, "plan-owner-3@test.com", "63000003")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "A borrar"}
	require.NoError(t, planDao.Create(nil, p))
	require.NoError(t, dayDao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "rest"},
	}))

	err := planDao.Delete(nil, p.ID)

	require.NoError(t, err)
	found, findErr := planDao.FindByID(nil, p.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
	days, daysErr := dayDao.FindByPlan(nil, p.ID)
	require.NoError(t, daysErr)
	assert.Empty(t, days)
}
```

`cmd/api/daos/training_plan_dao.go`:
```go
package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type TrainingPlanDaoInterface interface {
	Create(ctx *gin.Context, p *dbs.TrainingPlan) error
	FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error)
	Update(ctx *gin.Context, p *dbs.TrainingPlan) error
	Delete(ctx *gin.Context, id int64) error
}

type trainingPlanDao struct {
	DB *gorm.DB
}

func NewTrainingPlanDao(database *gorm.DB) TrainingPlanDaoInterface {
	return &trainingPlanDao{DB: database}
}

func (d *trainingPlanDao) Create(ctx *gin.Context, p *dbs.TrainingPlan) error {
	return d.DB.Create(p).Error
}

func (d *trainingPlanDao) FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
	var p dbs.TrainingPlan
	err := d.DB.Where("id = ?", id).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding training plan: %w", err)
	}
	return &p, nil
}

func (d *trainingPlanDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) {
	var plans []dbs.TrainingPlan
	err := d.DB.Where("owner_id = ?", ownerID).Order("id").Find(&plans).Error
	if err != nil {
		return nil, fmt.Errorf("error listing training plans: %w", err)
	}
	return plans, nil
}

func (d *trainingPlanDao) Update(ctx *gin.Context, p *dbs.TrainingPlan) error {
	return d.DB.Model(&dbs.TrainingPlan{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"name":        p.Name,
		"description": p.Description,
	}).Error
}

// Delete es físico — sin caducidad ni soft-delete para TrainingPlan (D1).
// Borra sus PlanDay en la misma transacción (sin FK física en este repo).
func (d *trainingPlanDao) Delete(ctx *gin.Context, id int64) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", id).Delete(&dbs.PlanDay{}).Error; err != nil {
			return fmt.Errorf("error deleting plan days: %w", err)
		}
		if err := tx.Delete(&dbs.TrainingPlan{}, id).Error; err != nil {
			return fmt.Errorf("error deleting training plan: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 2: Test + implementación de `PlanDayDaoInterface`**

`cmd/api/daos/plan_day_dao_test.go`:
```go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestPlanDayDao_ImplementsInterface(t *testing.T) {
	dao := NewPlanDayDao(&gorm.DB{})
	var iface PlanDayDaoInterface = dao
	_ = iface
}

func TestPlanDayDao_ReplaceForPlan_OrderedBySequence(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dao := NewPlanDayDao(db)
	owner := persistUser(db, "planday-owner-1@test.com", "64000001")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Con días"}
	require.NoError(t, planDao.Create(nil, p))

	err := dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 2, Kind: "rest"},
		{SequenceNo: 1, Kind: "other", OtherName: strPtr("Descanso activo")},
	})

	require.NoError(t, err)
	days, findErr := dao.FindByPlan(nil, p.ID)
	require.NoError(t, findErr)
	require.Len(t, days, 2)
	assert.Equal(t, 1, days[0].SequenceNo)
	assert.Equal(t, 2, days[1].SequenceNo)
}

func TestPlanDayDao_ReplaceForPlan_ReplacesEntireSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	planDao := NewTrainingPlanDao(db)
	dao := NewPlanDayDao(db)
	owner := persistUser(db, "planday-owner-2@test.com", "64000002")
	p := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "A reemplazar"}
	require.NoError(t, planDao.Create(nil, p))
	require.NoError(t, dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 2, Kind: "rest"}, {SequenceNo: 3, Kind: "rest"},
	}))

	err := dao.ReplaceForPlan(nil, p.ID, []dbs.PlanDay{
		{SequenceNo: 1, Kind: "rest"},
	})

	require.NoError(t, err)
	days, findErr := dao.FindByPlan(nil, p.ID)
	require.NoError(t, findErr)
	assert.Len(t, days, 1)
}

func strPtr(s string) *string { return &s }
```

`cmd/api/daos/plan_day_dao.go`:
```go
package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PlanDayDaoInterface interface {
	FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error)
	ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error
}

type planDayDao struct {
	DB *gorm.DB
}

func NewPlanDayDao(database *gorm.DB) PlanDayDaoInterface {
	return &planDayDao{DB: database}
}

func (d *planDayDao) FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
	var days []dbs.PlanDay
	err := d.DB.Where("plan_id = ?", planID).Order("sequence_no").Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error listing plan days: %w", err)
	}
	return days, nil
}

// ReplaceForPlan borra el set anterior y crea el nuevo en una transacción —
// mismo criterio que SessionExerciseDao.ReplaceForSession.
func (d *planDayDao) ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&dbs.PlanDay{}).Error; err != nil {
			return fmt.Errorf("error clearing plan days: %w", err)
		}
		for i := range rows {
			rows[i].ID = 0
			rows[i].PlanID = planID
			if err := tx.Create(&rows[i]).Error; err != nil {
				return fmt.Errorf("error creating plan day: %w", err)
			}
		}
		return nil
	})
}
```

**Nota para quien implemente**: si `strPtr` ya existe en otro `_test.go` del paquete `daos` (revisar con `grep -rn "func strPtr" cmd/api/daos/`), no redeclarar — Go no permite dos funciones con el mismo nombre en el mismo paquete. Usar el helper existente o renombrar el propio a algo único (ej. `planDayStrPtr`).

- [ ] **Step 3: Correr los tests, verificar que pasan**

Run: `go test ./cmd/api/daos/... -run "TestTrainingPlanDao|TestPlanDayDao" -v -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/daos/training_plan_dao.go cmd/api/daos/training_plan_dao_test.go cmd/api/daos/plan_day_dao.go cmd/api/daos/plan_day_dao_test.go
git commit -m "feat(catalog): add TrainingPlanDao and PlanDayDao"
```

---

### Task 6: ExerciseService

**Files:**
- Create: `cmd/api/services/exercise_service.go`
- Test: `cmd/api/services/exercise_service_test.go`

**Interfaces:**
- Consumes: `daos.ExerciseDaoInterface` (Task 3), `exercise.ExerciseRequest`/`ExerciseResponse` (Task 2).
- Produces: `ExerciseServiceInterface{Create(ctx,callerID,req),Update(ctx,id,callerID,req),Delete(ctx,id,callerID),Clone(ctx,id,callerID),Get(ctx,id),List(ctx,ownerID)}`, sentinels `ErrExerciseNotFound`, `ErrExerciseInvalidKind`, `ErrExerciseInvalidIntensity`, `ErrExerciseInvalidMuscleGroup`, `ErrCatalogForbidden` (este último compartido, lo declaran también Session/TrainingPlan services en sus propios archivos — Go permite `var X = errors.New(...)` en un solo archivo del paquete; **declararlo únicamente acá**, Tasks 8 y 9 lo consumen sin redeclarar).

- [ ] **Step 1: Escribir el test (mocks)**

`cmd/api/services/exercise_service_test.go`:
```go
package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/exercise"
)

type mockExerciseDao struct {
	createFn     func(ctx *gin.Context, e *dbs.Exercise) error
	findByIDFn   func(ctx *gin.Context, id int64) (*dbs.Exercise, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error)
	updateFn     func(ctx *gin.Context, e *dbs.Exercise) error
	softDeleteFn func(ctx *gin.Context, id int64) error
}

func (m *mockExerciseDao) Create(ctx *gin.Context, e *dbs.Exercise) error {
	if m.createFn != nil {
		return m.createFn(ctx, e)
	}
	e.ID = 1
	return nil
}
func (m *mockExerciseDao) FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockExerciseDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Exercise, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockExerciseDao) Update(ctx *gin.Context, e *dbs.Exercise) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, e)
	}
	return nil
}
func (m *mockExerciseDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestExerciseService_Create_Success(t *testing.T) {
	dao := &mockExerciseDao{}
	svc := NewExerciseService(dao)

	resp, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "Trote", Kind: "jogging"})

	require.NoError(t, err)
	assert.Equal(t, "Trote", resp.Name)
}

func TestExerciseService_Create_InvalidKind(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{})

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "flying"})

	assert.ErrorIs(t, err, ErrExerciseInvalidKind)
}

func TestExerciseService_Create_OwnerMismatch(t *testing.T) {
	svc := NewExerciseService(&mockExerciseDao{})

	_, err := svc.Create(nil, 7, exercise.ExerciseRequest{OwnerID: 99, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Update_NotFound(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewExerciseService(dao)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrExerciseNotFound)
}

func TestExerciseService_Update_Forbidden(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 99}, nil
	}}
	svc := NewExerciseService(dao)

	_, err := svc.Update(nil, 1, 7, exercise.ExerciseRequest{OwnerID: 7, Name: "X", Kind: "running"})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestExerciseService_Clone_Success(t *testing.T) {
	original := &dbs.Exercise{ID: 1, OwnerID: 7, Name: "Original", Kind: "running"}
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return original, nil }}
	svc := NewExerciseService(dao)

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
}

func TestExerciseService_Delete_Success(t *testing.T) {
	dao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id, OwnerID: 7}, nil
	}}
	svc := NewExerciseService(dao)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestExerciseService -v -count=1`
Expected: FAIL — `NewExerciseService` no existe.

- [ ] **Step 3: Implementar el service**

`cmd/api/services/exercise_service.go`:
```go
package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrExerciseNotFound          = errors.New("ejercicio no encontrado")
	ErrExerciseInvalidKind       = errors.New("kind inválido")
	ErrExerciseInvalidIntensity  = errors.New("intensity inválido")
	ErrExerciseInvalidMuscleGroup = errors.New("muscle_group inválido")
	// ErrCatalogForbidden es compartido por ExerciseService/SessionService/
	// TrainingPlanService (D9) — declarado una sola vez acá.
	ErrCatalogForbidden = errors.New("no autorizado")
)

type ExerciseServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error)
	Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error)
}

type exerciseService struct {
	exerciseDao daos.ExerciseDaoInterface
}

func NewExerciseService(exerciseDao daos.ExerciseDaoInterface) ExerciseServiceInterface {
	return &exerciseService{exerciseDao: exerciseDao}
}

func validateExerciseRequest(req exercise.ExerciseRequest) error {
	if !constants.IsValidExerciseKind(req.Kind) {
		return ErrExerciseInvalidKind
	}
	if req.Intensity != nil && !constants.IsValidExerciseIntensity(*req.Intensity) {
		return ErrExerciseInvalidIntensity
	}
	if req.MuscleGroup != nil && !constants.IsValidMuscleGroup(*req.MuscleGroup) {
		return ErrExerciseInvalidMuscleGroup
	}
	return nil
}

func (s *exerciseService) Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := validateExerciseRequest(req); err != nil {
		return nil, err
	}
	e := &dbs.Exercise{
		OwnerID: req.OwnerID, Name: req.Name, Description: req.Description, Kind: req.Kind,
		Intensity: req.Intensity, Minutes: req.Minutes, DistanceM: req.DistanceM,
		SpeedKph: req.SpeedKph, MuscleGroup: req.MuscleGroup,
	}
	if err := s.exerciseDao.Create(ctx, e); err != nil {
		customlogger.Error(ctx, "error creating exercise", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear ejercicio")
	}
	return toExerciseResponse(e), nil
}

func (s *exerciseService) Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar ejercicio")
	}
	if existing == nil {
		return nil, ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := validateExerciseRequest(req); err != nil {
		return nil, err
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.Kind = req.Kind
	existing.Intensity = req.Intensity
	existing.Minutes = req.Minutes
	existing.DistanceM = req.DistanceM
	existing.SpeedKph = req.SpeedKph
	existing.MuscleGroup = req.MuscleGroup
	if err := s.exerciseDao.Update(ctx, existing); err != nil {
		customlogger.Error(ctx, "error updating exercise", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar ejercicio")
	}
	return toExerciseResponse(existing), nil
}

func (s *exerciseService) Delete(ctx *gin.Context, id, callerID int64) error {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar ejercicio")
	}
	if existing == nil {
		return ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.exerciseDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error soft-deleting exercise", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar ejercicio")
	}
	return nil
}

func (s *exerciseService) Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error) {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar ejercicio")
	}
	if existing == nil {
		return nil, ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	clone := &dbs.Exercise{
		OwnerID: existing.OwnerID, Name: existing.Name + " (copia)", Description: existing.Description,
		Kind: existing.Kind, Intensity: existing.Intensity, Minutes: existing.Minutes,
		DistanceM: existing.DistanceM, SpeedKph: existing.SpeedKph, MuscleGroup: existing.MuscleGroup,
	}
	if err := s.exerciseDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error cloning exercise", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar ejercicio")
	}
	return toExerciseResponse(clone), nil
}

func (s *exerciseService) Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
	e, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar ejercicio")
	}
	if e == nil {
		return nil, ErrExerciseNotFound
	}
	return toExerciseResponse(e), nil
}

func (s *exerciseService) List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error) {
	exercises, err := s.exerciseDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing exercises", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar ejercicios")
	}
	responses := make([]exercise.ExerciseResponse, len(exercises))
	for i := range exercises {
		responses[i] = *toExerciseResponse(&exercises[i])
	}
	return responses, nil
}

func toExerciseResponse(e *dbs.Exercise) *exercise.ExerciseResponse {
	return &exercise.ExerciseResponse{
		ID: e.ID, OwnerID: e.OwnerID, Name: e.Name, Description: e.Description, Kind: e.Kind,
		Intensity: e.Intensity, Minutes: e.Minutes, DistanceM: e.DistanceM, SpeedKph: e.SpeedKph,
		MuscleGroup: e.MuscleGroup, VideoURL: e.VideoURL, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/exercise_service.go cmd/api/services/exercise_service_test.go
git commit -m "feat(catalog): add ExerciseService with owner authorization"
```

---

### Task 7: SessionService

**Files:**
- Create: `cmd/api/services/session_service.go`
- Test: `cmd/api/services/session_service_test.go`

**Interfaces:**
- Consumes: `daos.SessionDaoInterface`/`SessionExerciseDaoInterface` (Task 4), `daos.ExerciseDaoInterface` (Task 3, para validar `exercise_id`), `ErrCatalogForbidden` (Task 6), `session.*` DTOs (Task 2).
- Produces: `SessionServiceInterface{Create(ctx,callerID,req),Update(ctx,id,callerID,req),Delete(ctx,id,callerID),Clone(ctx,id,callerID),Get(ctx,id),List(ctx,ownerID)}`, sentinels `ErrSessionNotFound`, `ErrSessionMissingRole`, `ErrSessionInvalidRole`, `ErrSessionExerciseNotFound`.

- [ ] **Step 1: Escribir el test**

`cmd/api/services/session_service_test.go`:
```go
package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
)

type mockSessionDao struct {
	createFn      func(ctx *gin.Context, s *dbs.Session) error
	findByIDFn    func(ctx *gin.Context, id int64) (*dbs.Session, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error)
	updateFn      func(ctx *gin.Context, s *dbs.Session) error
	softDeleteFn  func(ctx *gin.Context, id int64) error
}

func (m *mockSessionDao) Create(ctx *gin.Context, s *dbs.Session) error {
	if m.createFn != nil {
		return m.createFn(ctx, s)
	}
	s.ID = 1
	return nil
}
func (m *mockSessionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Session, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockSessionDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockSessionDao) Update(ctx *gin.Context, s *dbs.Session) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, s)
	}
	return nil
}
func (m *mockSessionDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

type mockSessionExerciseDao struct {
	findBySessionFn    func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error)
	replaceForSessionFn func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error
}

func (m *mockSessionExerciseDao) FindBySession(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
	if m.findBySessionFn != nil {
		return m.findBySessionFn(ctx, sessionID)
	}
	return nil, nil
}
func (m *mockSessionExerciseDao) ReplaceForSession(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
	if m.replaceForSessionFn != nil {
		return m.replaceForSessionFn(ctx, sessionID, rows)
	}
	return nil
}

func validSessionExercises() []session.SessionExerciseRequest {
	return []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"},
		{ExerciseID: 2, Role: "main"},
		{ExerciseID: 3, Role: "cooldown"},
	}
}

func TestSessionService_Create_Success(t *testing.T) {
	sessionDao := &mockSessionDao{}
	sessionExerciseDao := &mockSessionExerciseDao{}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)

	resp, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Completa", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Completa", resp.Name)
}

func TestSessionService_Create_MissingRole(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Incompleta", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"},
	}})

	assert.ErrorIs(t, err, ErrSessionMissingRole)
}

func TestSessionService_Create_InvalidRole(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "flying"},
	}})

	assert.ErrorIs(t, err, ErrSessionInvalidRole)
}

func TestSessionService_Create_ExerciseNotFound(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
}

func TestSessionService_Create_OwnerMismatch(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{})

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 99, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Clone_DeepCopiesExercises(t *testing.T) {
	original := &dbs.Session{ID: 1, OwnerID: 7, Name: "Original"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return original, nil }}
	replaced := false
	sessionExerciseDao := &mockSessionExerciseDao{
		findBySessionFn: func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error) {
			return []dbs.SessionExercise{{ExerciseID: 1, Role: "warmup"}}, nil
		},
		replaceForSessionFn: func(ctx *gin.Context, sessionID int64, rows []dbs.SessionExercise) error {
			replaced = true
			return nil
		},
	}
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{})

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
	assert.True(t, replaced)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestSessionService -v -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar el service**

`cmd/api/services/session_service.go`:
```go
package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrSessionNotFound         = errors.New("sesión no encontrada")
	ErrSessionMissingRole      = errors.New("la sesión debe tener al menos un ejercicio de cada rol")
	ErrSessionInvalidRole      = errors.New("role inválido")
	ErrSessionExerciseNotFound = errors.New("ejercicio referenciado no encontrado")
)

type SessionServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error)
	Get(ctx *gin.Context, id int64) (*session.SessionResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error)
}

type sessionService struct {
	sessionDao         daos.SessionDaoInterface
	sessionExerciseDao daos.SessionExerciseDaoInterface
	exerciseDao        daos.ExerciseDaoInterface
}

func NewSessionService(sessionDao daos.SessionDaoInterface, sessionExerciseDao daos.SessionExerciseDaoInterface, exerciseDao daos.ExerciseDaoInterface) SessionServiceInterface {
	return &sessionService{sessionDao: sessionDao, sessionExerciseDao: sessionExerciseDao, exerciseDao: exerciseDao}
}

func (s *sessionService) validateExercises(ctx *gin.Context, items []session.SessionExerciseRequest) error {
	roleCounts := map[string]int{}
	for _, item := range items {
		if !constants.IsValidSessionExerciseRole(item.Role) {
			return ErrSessionInvalidRole
		}
		roleCounts[item.Role]++
		ex, err := s.exerciseDao.FindByID(ctx, item.ExerciseID)
		if err != nil {
			return fmt.Errorf("error al validar ejercicios de la sesión")
		}
		if ex == nil {
			return ErrSessionExerciseNotFound
		}
	}
	for _, role := range constants.GetValidSessionExerciseRoles() {
		if roleCounts[role] < 1 {
			return ErrSessionMissingRole
		}
	}
	return nil
}

func toSessionExerciseRows(items []session.SessionExerciseRequest) []dbs.SessionExercise {
	rows := make([]dbs.SessionExercise, len(items))
	for i, item := range items {
		repeatCount := 1
		if item.RepeatCount != nil {
			repeatCount = *item.RepeatCount
		}
		restMinutes := 0
		if item.RestMinutes != nil {
			restMinutes = *item.RestMinutes
		}
		rows[i] = dbs.SessionExercise{ExerciseID: item.ExerciseID, Role: item.Role, RepeatCount: repeatCount, RestMinutes: restMinutes}
	}
	return rows
}

func (s *sessionService) toResponse(ctx *gin.Context, sessionDB *dbs.Session) (*session.SessionResponse, error) {
	rows, err := s.sessionExerciseDao.FindBySession(ctx, sessionDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading session exercises", err, customlogger.TagMethod("toResponse"))
		return nil, fmt.Errorf("error al armar la respuesta de la sesión")
	}
	exercises := make([]session.SessionExerciseResponse, len(rows))
	for i, r := range rows {
		exercises[i] = session.SessionExerciseResponse{ID: r.ID, ExerciseID: r.ExerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	return &session.SessionResponse{
		ID: sessionDB.ID, OwnerID: sessionDB.OwnerID, Name: sessionDB.Name, Description: sessionDB.Description,
		Exercises: exercises, CreatedAt: sessionDB.CreatedAt, UpdatedAt: sessionDB.UpdatedAt,
	}, nil
}

func (s *sessionService) Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := s.validateExercises(ctx, req.Exercises); err != nil {
		return nil, err
	}
	sessionDB := &dbs.Session{OwnerID: req.OwnerID, Name: req.Name, Description: req.Description}
	if err := s.sessionDao.Create(ctx, sessionDB); err != nil {
		customlogger.Error(ctx, "error creating session", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear sesión")
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, sessionDB.ID, toSessionExerciseRows(req.Exercises)); err != nil {
		customlogger.Error(ctx, "error setting session exercises", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear sesión")
	}
	return s.toResponse(ctx, sessionDB)
}

func (s *sessionService) Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	if existing == nil {
		return nil, ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := s.validateExercises(ctx, req.Exercises); err != nil {
		return nil, err
	}
	existing.Name = req.Name
	existing.Description = req.Description
	if err := s.sessionDao.Update(ctx, existing); err != nil {
		customlogger.Error(ctx, "error updating session", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, id, toSessionExerciseRows(req.Exercises)); err != nil {
		customlogger.Error(ctx, "error replacing session exercises", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	return s.toResponse(ctx, existing)
}

func (s *sessionService) Delete(ctx *gin.Context, id, callerID int64) error {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar sesión")
	}
	if existing == nil {
		return ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.sessionDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error soft-deleting session", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar sesión")
	}
	return nil
}

func (s *sessionService) Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error) {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	if existing == nil {
		return nil, ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	rows, err := s.sessionExerciseDao.FindBySession(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error loading session exercises", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	clone := &dbs.Session{OwnerID: existing.OwnerID, Name: existing.Name + " (copia)", Description: existing.Description}
	if err := s.sessionDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error cloning session", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	clonedRows := make([]dbs.SessionExercise, len(rows))
	for i, r := range rows {
		clonedRows[i] = dbs.SessionExercise{ExerciseID: r.ExerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, clone.ID, clonedRows); err != nil {
		customlogger.Error(ctx, "error setting cloned session exercises", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	return s.toResponse(ctx, clone)
}

func (s *sessionService) Get(ctx *gin.Context, id int64) (*session.SessionResponse, error) {
	sessionDB, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar sesión")
	}
	if sessionDB == nil {
		return nil, ErrSessionNotFound
	}
	return s.toResponse(ctx, sessionDB)
}

func (s *sessionService) List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error) {
	sessions, err := s.sessionDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing sessions", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar sesiones")
	}
	responses := make([]session.SessionResponse, len(sessions))
	for i := range sessions {
		resp, err := s.toResponse(ctx, &sessions[i])
		if err != nil {
			return nil, err
		}
		responses[i] = *resp
	}
	return responses, nil
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/session_service.go cmd/api/services/session_service_test.go
git commit -m "feat(catalog): add SessionService with role validation and deep-copy clone"
```

---

### Task 8: TrainingPlanService

**Files:**
- Create: `cmd/api/services/training_plan_service.go`
- Test: `cmd/api/services/training_plan_service_test.go`

**Interfaces:**
- Consumes: `daos.TrainingPlanDaoInterface`/`PlanDayDaoInterface` (Task 5), `daos.SessionDaoInterface` (Task 4, para validar `session_id`), `ErrCatalogForbidden` (Task 6), `trainingplan.*` DTOs (Task 2).
- Produces: `TrainingPlanServiceInterface{Create(ctx,callerID,req),Update(ctx,id,callerID,req),Delete(ctx,id,callerID),Clone(ctx,id,callerID),Get(ctx,id),List(ctx,ownerID)}`, sentinels `ErrPlanNotFound`, `ErrPlanInvalidDayCount`, `ErrPlanInvalidSequence`, `ErrPlanInvalidDayKind`, `ErrPlanDayFieldMismatch`, `ErrPlanSessionNotFound`, `ErrPlanInvalidTimeFormat`.

- [ ] **Step 1: Escribir el test**

`cmd/api/services/training_plan_service_test.go`:
```go
package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type mockTrainingPlanDao struct {
	createFn      func(ctx *gin.Context, p *dbs.TrainingPlan) error
	findByIDFn    func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error)
	findByOwnerFn func(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error)
	updateFn      func(ctx *gin.Context, p *dbs.TrainingPlan) error
	deleteFn      func(ctx *gin.Context, id int64) error
}

func (m *mockTrainingPlanDao) Create(ctx *gin.Context, p *dbs.TrainingPlan) error {
	if m.createFn != nil {
		return m.createFn(ctx, p)
	}
	p.ID = 1
	return nil
}
func (m *mockTrainingPlanDao) FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockTrainingPlanDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) {
	if m.findByOwnerFn != nil {
		return m.findByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}
func (m *mockTrainingPlanDao) Update(ctx *gin.Context, p *dbs.TrainingPlan) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}
func (m *mockTrainingPlanDao) Delete(ctx *gin.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

type mockPlanDayDao struct {
	findByPlanFn     func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error)
	replaceForPlanFn func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error
}

func (m *mockPlanDayDao) FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
	if m.findByPlanFn != nil {
		return m.findByPlanFn(ctx, planID)
	}
	return nil, nil
}
func (m *mockPlanDayDao) ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
	if m.replaceForPlanFn != nil {
		return m.replaceForPlanFn(ctx, planID, rows)
	}
	return nil
}

func validPlanDays() []trainingplan.PlanDayRequest {
	return []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
		{SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación")},
	}
}

func strPtrTP(s string) *string { return &s }

func TestTrainingPlanService_Create_Success(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	resp, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: validPlanDays()})

	require.NoError(t, err)
	assert.Equal(t, "Plan", resp.Name)
}

func TestTrainingPlanService_Create_TooFewDays(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidDayCount)
}

func TestTrainingPlanService_Create_SequenceGap(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 3, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanInvalidSequence)
}

func TestTrainingPlanService_Create_TrainingWithoutSessionID(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "training"}, {SequenceNo: 2, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanDayFieldMismatch)
}

func TestTrainingPlanService_Create_TrainingSessionNotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, sessionDao)
	sessionID := int64(5)

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "training", SessionID: &sessionID}, {SequenceNo: 2, Kind: "rest"},
	}})

	assert.ErrorIs(t, err, ErrPlanSessionNotFound)
}

func TestTrainingPlanService_Create_OwnerMismatch(t *testing.T) {
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, &mockSessionDao{})

	_, err := svc.Create(nil, 7, trainingplan.TrainingPlanRequest{OwnerID: 99, Name: "Plan", Days: validPlanDays()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestTrainingPlanService_Update_PartialWithoutDays(t *testing.T) {
	existing := &dbs.TrainingPlan{ID: 1, OwnerID: 7, Name: "Viejo"}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) { return existing, nil }}
	dayDaoCalled := false
	dayDao := &mockPlanDayDao{replaceForPlanFn: func(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
		dayDaoCalled = true
		return nil
	}}
	svc := NewTrainingPlanService(planDao, dayDao, &mockSessionDao{})
	newName := "Nuevo"

	_, err := svc.Update(nil, 1, 7, trainingplan.TrainingPlanUpdateRequest{Name: &newName})

	require.NoError(t, err)
	assert.False(t, dayDaoCalled, "no debería tocar los días si no vinieron en el body")
}

func TestTrainingPlanService_Delete_Forbidden(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 99}, nil
	}}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{})

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestTrainingPlanService -v -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar el service**

`cmd/api/services/training_plan_service.go`:
```go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrPlanNotFound          = errors.New("plan no encontrado")
	ErrPlanInvalidDayCount   = errors.New("el plan debe tener entre 2 y 31 días")
	ErrPlanInvalidSequence   = errors.New("sequence_no debe cubrir 1..N sin huecos ni repetidos")
	ErrPlanInvalidDayKind    = errors.New("kind de día inválido")
	ErrPlanDayFieldMismatch  = errors.New("combinación de campos inválida para el kind del día")
	ErrPlanSessionNotFound   = errors.New("session_id referenciado no encontrado")
	ErrPlanInvalidTimeFormat = errors.New("default_time debe tener formato HH:MM")
)

type TrainingPlanServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error)
	Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error)
}

type trainingPlanService struct {
	trainingPlanDao daos.TrainingPlanDaoInterface
	planDayDao      daos.PlanDayDaoInterface
	sessionDao      daos.SessionDaoInterface
}

func NewTrainingPlanService(trainingPlanDao daos.TrainingPlanDaoInterface, planDayDao daos.PlanDayDaoInterface, sessionDao daos.SessionDaoInterface) TrainingPlanServiceInterface {
	return &trainingPlanService{trainingPlanDao: trainingPlanDao, planDayDao: planDayDao, sessionDao: sessionDao}
}

func (s *trainingPlanService) validateAndBuildDays(ctx *gin.Context, days []trainingplan.PlanDayRequest) ([]dbs.PlanDay, error) {
	n := len(days)
	if n < 2 || n > 31 {
		return nil, ErrPlanInvalidDayCount
	}
	seen := make(map[int]bool, n)
	for _, d := range days {
		if d.SequenceNo < 1 || d.SequenceNo > n || seen[d.SequenceNo] {
			return nil, ErrPlanInvalidSequence
		}
		seen[d.SequenceNo] = true
	}

	rows := make([]dbs.PlanDay, n)
	for i, d := range days {
		if !constants.IsValidPlanDayKind(d.Kind) {
			return nil, ErrPlanInvalidDayKind
		}
		switch d.Kind {
		case string(constants.PlanDayKindTraining):
			if d.SessionID == nil || d.OtherName != nil {
				return nil, ErrPlanDayFieldMismatch
			}
			sessionDB, err := s.sessionDao.FindByID(ctx, *d.SessionID)
			if err != nil {
				return nil, fmt.Errorf("error al validar sesión del día")
			}
			if sessionDB == nil {
				return nil, ErrPlanSessionNotFound
			}
		case string(constants.PlanDayKindOther):
			if d.OtherName == nil || d.SessionID != nil {
				return nil, ErrPlanDayFieldMismatch
			}
		case string(constants.PlanDayKindRest):
			if d.OtherName != nil || d.SessionID != nil {
				return nil, ErrPlanDayFieldMismatch
			}
		}

		defaultPresencial := false
		if d.DefaultPresencial != nil {
			defaultPresencial = *d.DefaultPresencial
		}

		row := dbs.PlanDay{
			SequenceNo:        d.SequenceNo,
			Kind:              d.Kind,
			OtherName:         d.OtherName,
			SessionID:         d.SessionID,
			DefaultPresencial: defaultPresencial,
		}

		if defaultPresencial {
			if d.DefaultTime == nil || d.DefaultLocation == nil {
				return nil, ErrPlanDayFieldMismatch
			}
			parsedTime, err := time.Parse("15:04", *d.DefaultTime)
			if err != nil {
				return nil, ErrPlanInvalidTimeFormat
			}
			row.DefaultTime = &parsedTime
			locationJSON, err := json.Marshal(d.DefaultLocation)
			if err != nil {
				return nil, fmt.Errorf("error al serializar la ubicación del día")
			}
			locationStr := string(locationJSON)
			row.DefaultLocation = &locationStr
		}

		rows[i] = row
	}
	return rows, nil
}

func toPlanDayResponse(d dbs.PlanDay) trainingplan.PlanDayResponse {
	resp := trainingplan.PlanDayResponse{
		ID: d.ID, SequenceNo: d.SequenceNo, Kind: d.Kind, OtherName: d.OtherName,
		SessionID: d.SessionID, DefaultPresencial: d.DefaultPresencial,
	}
	if d.DefaultTime != nil {
		formatted := d.DefaultTime.Format("15:04")
		resp.DefaultTime = &formatted
	}
	if d.DefaultLocation != nil {
		var loc trainingplan.Location
		if err := json.Unmarshal([]byte(*d.DefaultLocation), &loc); err == nil {
			resp.DefaultLocation = &loc
		}
	}
	return resp
}

func (s *trainingPlanService) toResponse(ctx *gin.Context, planDB *dbs.TrainingPlan) (*trainingplan.TrainingPlanResponse, error) {
	days, err := s.planDayDao.FindByPlan(ctx, planDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading plan days", err, customlogger.TagMethod("toResponse"))
		return nil, fmt.Errorf("error al armar la respuesta del plan")
	}
	dayResponses := make([]trainingplan.PlanDayResponse, len(days))
	for i, d := range days {
		dayResponses[i] = toPlanDayResponse(d)
	}
	return &trainingplan.TrainingPlanResponse{
		ID: planDB.ID, OwnerID: planDB.OwnerID, Name: planDB.Name, Description: planDB.Description,
		Days: dayResponses, CreatedAt: planDB.CreatedAt, UpdatedAt: planDB.UpdatedAt,
	}, nil
}

func (s *trainingPlanService) Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	rows, err := s.validateAndBuildDays(ctx, req.Days)
	if err != nil {
		return nil, err
	}
	planDB := &dbs.TrainingPlan{OwnerID: req.OwnerID, Name: req.Name, Description: req.Description}
	if err := s.trainingPlanDao.Create(ctx, planDB); err != nil {
		customlogger.Error(ctx, "error creating training plan", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear plan")
	}
	if err := s.planDayDao.ReplaceForPlan(ctx, planDB.ID, rows); err != nil {
		customlogger.Error(ctx, "error setting plan days", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear plan")
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar plan")
	}
	if planDB == nil {
		return nil, ErrPlanNotFound
	}
	if planDB.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if req.Name != nil {
		planDB.Name = *req.Name
	}
	if req.Description != nil {
		planDB.Description = req.Description
	}
	if err := s.trainingPlanDao.Update(ctx, planDB); err != nil {
		customlogger.Error(ctx, "error updating training plan", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar plan")
	}
	if req.Days != nil {
		rows, err := s.validateAndBuildDays(ctx, *req.Days)
		if err != nil {
			return nil, err
		}
		if err := s.planDayDao.ReplaceForPlan(ctx, id, rows); err != nil {
			customlogger.Error(ctx, "error replacing plan days", err, customlogger.TagMethod("Update"))
			return nil, fmt.Errorf("error al editar plan")
		}
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) Delete(ctx *gin.Context, id, callerID int64) error {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar plan")
	}
	if planDB == nil {
		return ErrPlanNotFound
	}
	if planDB.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.trainingPlanDao.Delete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting training plan", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar plan")
	}
	return nil
}

func (s *trainingPlanService) Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
	original, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	if original == nil {
		return nil, ErrPlanNotFound
	}
	if original.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	originalDays, err := s.planDayDao.FindByPlan(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error loading plan days", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	clone := &dbs.TrainingPlan{OwnerID: original.OwnerID, Name: original.Name + " (copia)", Description: original.Description}
	if err := s.trainingPlanDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error creating cloned plan", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	clonedDays := make([]dbs.PlanDay, len(originalDays))
	for i, d := range originalDays {
		clonedDays[i] = dbs.PlanDay{
			SequenceNo: d.SequenceNo, Kind: d.Kind, OtherName: d.OtherName, SessionID: d.SessionID,
			DefaultPresencial: d.DefaultPresencial, DefaultTime: d.DefaultTime, DefaultLocation: d.DefaultLocation,
		}
	}
	if err := s.planDayDao.ReplaceForPlan(ctx, clone.ID, clonedDays); err != nil {
		customlogger.Error(ctx, "error setting cloned plan days", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	return s.toResponse(ctx, clone)
}

func (s *trainingPlanService) Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar plan")
	}
	if planDB == nil {
		return nil, ErrPlanNotFound
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error) {
	plans, err := s.trainingPlanDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing training plans", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar planes")
	}
	responses := make([]trainingplan.TrainingPlanResponse, len(plans))
	for i := range plans {
		resp, err := s.toResponse(ctx, &plans[i])
		if err != nil {
			return nil, err
		}
		responses[i] = *resp
	}
	return responses, nil
}
```

**Nota**: `strPtrTP` en el test duplica el propósito de `strPtr` de `cmd/api/daos/plan_day_dao_test.go`, pero están en paquetes distintos (`services` vs `daos`) — no hay colisión, no hace falta unificar.

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/training_plan_service.go cmd/api/services/training_plan_service_test.go
git commit -m "feat(catalog): add TrainingPlanService with day validation"
```

---

### Task 9: ExerciseController

**Files:**
- Create: `cmd/api/controllers/catalog_common.go`
- Create: `cmd/api/controllers/exercise_controller.go`
- Test: `cmd/api/controllers/exercise_controller_test.go`

**Interfaces:**
- Consumes: `ExerciseServiceInterface` (Task 6), `utils.GetAuthUserID` (ya existe en el repo, ver `join_request_controller.go`).
- Produces: `respondCatalogError(c, status, message)` (compartido, Tasks 10 y 11 lo consumen sin redeclarar); `ExerciseController{Create,Get,List,Update,Delete,Clone}`.

- [ ] **Step 1: Helper compartido de error**

`cmd/api/controllers/catalog_common.go`:
```go
package controllers

import "github.com/gin-gonic/gin"

// respondCatalogError responde con el shape {"message": "..."} pedido por el
// frontend para el dominio de catálogo/calendario — distinto del
// apierror.APIError SCREAMING_SNAKE del resto del backend (design.md D6 de
// catalogo-planes-entrenamiento).
func respondCatalogError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"message": message})
}
```

- [ ] **Step 2: Escribir el test del controller**

`cmd/api/controllers/exercise_controller_test.go`:
```go
package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockExerciseService struct {
	createFn func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error)
}

func (m *mockExerciseService) Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockExerciseService) Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockExerciseService) List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockExerciseService) Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockExerciseService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockExerciseService) Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupExerciseRouter(svc services.ExerciseServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewExerciseController(svc)
	r.POST("/exercises", ctrl.Create)
	r.GET("/exercises/:id", ctrl.Get)
	r.GET("/exercises", ctrl.List)
	r.PUT("/exercises/:id", ctrl.Update)
	r.DELETE("/exercises/:id", ctrl.Delete)
	r.POST("/exercises/:id/clone", ctrl.Clone)
	return r
}

func TestExerciseController_Create_Success(t *testing.T) {
	svc := &mockExerciseService{createFn: func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
		return &exercise.ExerciseResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupExerciseRouter(svc, 7)
	body, _ := json.Marshal(exercise.ExerciseRequest{OwnerID: 7, Name: "Trote", Kind: "jogging"})
	req := httptest.NewRequest(http.MethodPost, "/exercises", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestExerciseController_Create_Forbidden(t *testing.T) {
	svc := &mockExerciseService{createFn: func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
		return nil, services.ErrCatalogForbidden
	}}
	router := setupExerciseRouter(svc, 7)
	body, _ := json.Marshal(exercise.ExerciseRequest{OwnerID: 99, Name: "X", Kind: "running"})
	req := httptest.NewRequest(http.MethodPost, "/exercises", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestExerciseController_Get_NotFound(t *testing.T) {
	svc := &mockExerciseService{getFn: func(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
		return nil, services.ErrExerciseNotFound
	}}
	router := setupExerciseRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/exercises/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestExerciseController_Delete_Success(t *testing.T) {
	svc := &mockExerciseService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupExerciseRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/exercises/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
```

`utils.AuthUserIDKey` (`cmd/api/utils/authcontext.go`) es la key real que usa `AuthMiddleware()` en producción — el middleware de test de arriba ya la usa, no una key inventada.

- [ ] **Step 3: Implementar el controller**

`cmd/api/controllers/exercise_controller.go`:
```go
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type ExerciseController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type exerciseController struct {
	exerciseService services.ExerciseServiceInterface
}

func NewExerciseController(exerciseService services.ExerciseServiceInterface) ExerciseController {
	return &exerciseController{exerciseService: exerciseService}
}

func mapExerciseError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrExerciseNotFound):
		return http.StatusNotFound, "ejercicio no encontrado"
	case errors.Is(err, services.ErrExerciseInvalidKind):
		return http.StatusBadRequest, "kind inválido"
	case errors.Is(err, services.ErrExerciseInvalidIntensity):
		return http.StatusBadRequest, "intensity inválido"
	case errors.Is(err, services.ErrExerciseInvalidMuscleGroup):
		return http.StatusBadRequest, "muscle_group inválido"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondExerciseError(c *gin.Context, err error) {
	status, message := mapExerciseError(err)
	respondCatalogError(c, status, message)
}

func (ec *exerciseController) Create(c *gin.Context) {
	var req exercise.ExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Create(c, callerID, req)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (ec *exerciseController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := ec.exerciseService.Get(c, id)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := ec.exerciseService.List(c, ownerID)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req exercise.ExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Update(c, id, callerID, req)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := ec.exerciseService.Delete(c, id, callerID); err != nil {
		respondExerciseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ec *exerciseController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Clone(c, id, callerID)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
```

- [ ] **Step 4: Correr el test, ajustar la key de contexto si hace falta, verificar que pasa**

Run: `go test ./cmd/api/controllers/... -run TestExerciseController -v -count=1`
Expected: PASS (después de confirmar/ajustar la key real de `utils.GetAuthUserID` en el test, Step 2).

- [ ] **Step 5: Commit**

```bash
git add cmd/api/controllers/catalog_common.go cmd/api/controllers/exercise_controller.go cmd/api/controllers/exercise_controller_test.go
git commit -m "feat(catalog): add ExerciseController"
```

---

### Task 10: SessionController

**Files:**
- Create: `cmd/api/controllers/session_controller.go`
- Test: `cmd/api/controllers/session_controller_test.go`

**Interfaces:**
- Consumes: `SessionServiceInterface` (Task 7), `respondCatalogError` (Task 9).
- Produces: `SessionController{Create,Get,List,Update,Delete,Clone}`.

- [ ] **Step 1: Test del controller**

`cmd/api/controllers/session_controller_test.go` (mismo molde que `exercise_controller_test.go`, adaptado a `session.SessionRequest`/`SessionResponse` y a `services.SessionServiceInterface`):
```go
package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockSessionService struct {
	createFn func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*session.SessionResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error)
}

func (m *mockSessionService) Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockSessionService) Get(ctx *gin.Context, id int64) (*session.SessionResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockSessionService) List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockSessionService) Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockSessionService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockSessionService) Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupSessionRouter(svc services.SessionServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewSessionController(svc)
	r.POST("/sessions", ctrl.Create)
	r.GET("/sessions/:id", ctrl.Get)
	r.GET("/sessions", ctrl.List)
	r.PUT("/sessions/:id", ctrl.Update)
	r.DELETE("/sessions/:id", ctrl.Delete)
	r.POST("/sessions/:id/clone", ctrl.Clone)
	return r
}

func TestSessionController_Create_Success(t *testing.T) {
	svc := &mockSessionService{createFn: func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
		return &session.SessionResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupSessionRouter(svc, 7)
	body, _ := json.Marshal(session.SessionRequest{OwnerID: 7, Name: "Sesión", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"}, {ExerciseID: 3, Role: "cooldown"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestSessionController_Create_MissingRole(t *testing.T) {
	svc := &mockSessionService{createFn: func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
		return nil, services.ErrSessionMissingRole
	}}
	router := setupSessionRouter(svc, 7)
	body, _ := json.Marshal(session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{}})
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSessionController_Delete_Success(t *testing.T) {
	svc := &mockSessionService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupSessionRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/sessions/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
```

- [ ] **Step 2: Implementar el controller**

`cmd/api/controllers/session_controller.go`:
```go
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type SessionController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type sessionController struct {
	sessionService services.SessionServiceInterface
}

func NewSessionController(sessionService services.SessionServiceInterface) SessionController {
	return &sessionController{sessionService: sessionService}
}

func mapSessionError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrSessionNotFound):
		return http.StatusNotFound, "sesión no encontrada"
	case errors.Is(err, services.ErrSessionMissingRole):
		return http.StatusUnprocessableEntity, "la sesión debe tener al menos un ejercicio de cada rol"
	case errors.Is(err, services.ErrSessionInvalidRole):
		return http.StatusUnprocessableEntity, "role inválido"
	case errors.Is(err, services.ErrSessionExerciseNotFound):
		return http.StatusUnprocessableEntity, "ejercicio referenciado no encontrado"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondSessionError(c *gin.Context, err error) {
	status, message := mapSessionError(err)
	respondCatalogError(c, status, message)
}

func (sc *sessionController) Create(c *gin.Context) {
	var req session.SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Create(c, callerID, req)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (sc *sessionController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := sc.sessionService.Get(c, id)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := sc.sessionService.List(c, ownerID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req session.SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Update(c, id, callerID, req)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := sc.sessionService.Delete(c, id, callerID); err != nil {
		respondSessionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (sc *sessionController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Clone(c, id, callerID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
```

- [ ] **Step 3: Correr el test, verificar que pasa**

Run: `go test ./cmd/api/controllers/... -run TestSessionController -v -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/controllers/session_controller.go cmd/api/controllers/session_controller_test.go
git commit -m "feat(catalog): add SessionController"
```

---

### Task 11: TrainingPlanController

**Files:**
- Create: `cmd/api/controllers/training_plan_controller.go`
- Test: `cmd/api/controllers/training_plan_controller_test.go`

**Interfaces:**
- Consumes: `TrainingPlanServiceInterface` (Task 8), `respondCatalogError` (Task 9).
- Produces: `TrainingPlanController{Create,Get,List,Update,Delete,Clone}`.

- [ ] **Step 1: Test del controller** (mismo molde de Tasks 9/10, adaptado a `trainingplan.TrainingPlanRequest`/`TrainingPlanUpdateRequest`/`TrainingPlanResponse` y `services.TrainingPlanServiceInterface` — seguir exactamente la estructura de `session_controller_test.go`, cambiando los tipos y agregando un caso para el `Update` parcial sin `days`)

`cmd/api/controllers/training_plan_controller_test.go`:
```go
package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockTrainingPlanService struct {
	createFn func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error)
}

func (m *mockTrainingPlanService) Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockTrainingPlanService) Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockTrainingPlanService) List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockTrainingPlanService) Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockTrainingPlanService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockTrainingPlanService) Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupTrainingPlanRouter(svc services.TrainingPlanServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewTrainingPlanController(svc)
	r.POST("/training-plans", ctrl.Create)
	r.GET("/training-plans/:id", ctrl.Get)
	r.GET("/training-plans", ctrl.List)
	r.PUT("/training-plans/:id", ctrl.Update)
	r.DELETE("/training-plans/:id", ctrl.Delete)
	r.POST("/training-plans/:id/clone", ctrl.Clone)
	return r
}

func TestTrainingPlanController_Create_Success(t *testing.T) {
	svc := &mockTrainingPlanService{createFn: func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 2, Kind: "rest"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/training-plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTrainingPlanController_Create_InvalidDayCount(t *testing.T) {
	svc := &mockTrainingPlanService{createFn: func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
		return nil, services.ErrPlanInvalidDayCount
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{{SequenceNo: 1, Kind: "rest"}}})
	req := httptest.NewRequest(http.MethodPost, "/training-plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTrainingPlanController_Update_Partial(t *testing.T) {
	svc := &mockTrainingPlanService{updateFn: func(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: id, Name: *req.Name}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanUpdateRequest{Name: strPtrTPController("Nuevo nombre")})
	req := httptest.NewRequest(http.MethodPut, "/training-plans/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTrainingPlanController_Delete_Success(t *testing.T) {
	svc := &mockTrainingPlanService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/training-plans/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func strPtrTPController(s string) *string { return &s }
```

- [ ] **Step 2: Implementar el controller**

`cmd/api/controllers/training_plan_controller.go`:
```go
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type TrainingPlanController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type trainingPlanController struct {
	trainingPlanService services.TrainingPlanServiceInterface
}

func NewTrainingPlanController(trainingPlanService services.TrainingPlanServiceInterface) TrainingPlanController {
	return &trainingPlanController{trainingPlanService: trainingPlanService}
}

func mapTrainingPlanError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrPlanNotFound):
		return http.StatusNotFound, "plan no encontrado"
	case errors.Is(err, services.ErrPlanInvalidDayCount):
		return http.StatusUnprocessableEntity, "el plan debe tener entre 2 y 31 días"
	case errors.Is(err, services.ErrPlanInvalidSequence):
		return http.StatusUnprocessableEntity, "sequence_no debe cubrir 1..N sin huecos ni repetidos"
	case errors.Is(err, services.ErrPlanInvalidDayKind):
		return http.StatusUnprocessableEntity, "kind de día inválido"
	case errors.Is(err, services.ErrPlanDayFieldMismatch):
		return http.StatusUnprocessableEntity, "combinación de campos inválida para el kind del día"
	case errors.Is(err, services.ErrPlanSessionNotFound):
		return http.StatusUnprocessableEntity, "session_id referenciado no encontrado"
	case errors.Is(err, services.ErrPlanInvalidTimeFormat):
		return http.StatusUnprocessableEntity, "default_time debe tener formato HH:MM"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondTrainingPlanError(c *gin.Context, err error) {
	status, message := mapTrainingPlanError(err)
	respondCatalogError(c, status, message)
}

func (tc *trainingPlanController) Create(c *gin.Context) {
	var req trainingplan.TrainingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Create(c, callerID, req)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (tc *trainingPlanController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := tc.trainingPlanService.Get(c, id)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (tc *trainingPlanController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := tc.trainingPlanService.List(c, ownerID)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (tc *trainingPlanController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req trainingplan.TrainingPlanUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Update(c, id, callerID, req)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (tc *trainingPlanController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := tc.trainingPlanService.Delete(c, id, callerID); err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (tc *trainingPlanController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Clone(c, id, callerID)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
```

- [ ] **Step 3: Correr el test, verificar que pasa**

Run: `go test ./cmd/api/controllers/... -run TestTrainingPlanController -v -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/controllers/training_plan_controller.go cmd/api/controllers/training_plan_controller_test.go
git commit -m "feat(catalog): add TrainingPlanController"
```

---

### Task 12: Wiring (app.go + url_mappings.go)

**Files:**
- Modify: `cmd/api/app/app.go`
- Modify: `cmd/api/app/url_mappings.go`

**Interfaces:**
- Consumes: `daos.New{Exercise,Session,SessionExercise,TrainingPlan,PlanDay}Dao`, `services.New{Exercise,Session,TrainingPlan}Service`, `controllers.New{Exercise,Session,TrainingPlan}Controller` (Tasks 3-11).

- [ ] **Step 1: Agregar los campos y el wiring en `app.go`**

Buscar el bloque de `Application` struct (`grep -n "joinRequestController" cmd/api/app/app.go` para ubicar el patrón) y agregar, análogo a `joinRequestController`:
```go
exerciseController     controllers.ExerciseController
sessionController      controllers.SessionController
trainingPlanController controllers.TrainingPlanController
```

En `NewApplication()`, después del bloque `// Join Request flow`, agregar:
```go
// Catalog flow (Exercise/Session/TrainingPlan)
exerciseDao := daos.NewExerciseDao(db)
sessionDao := daos.NewSessionDao(db)
sessionExerciseDao := daos.NewSessionExerciseDao(db)
trainingPlanDao := daos.NewTrainingPlanDao(db)
planDayDao := daos.NewPlanDayDao(db)

exerciseService := services.NewExerciseService(exerciseDao)
exerciseController := controllers.NewExerciseController(exerciseService)

sessionService := services.NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)
sessionController := controllers.NewSessionController(sessionService)

trainingPlanService := services.NewTrainingPlanService(trainingPlanDao, planDayDao, sessionDao)
trainingPlanController := controllers.NewTrainingPlanController(trainingPlanService)
```

Y en el `return &Application{...}` (buscar `joinRequestController:` para ubicar el bloque), agregar:
```go
exerciseController:     exerciseController,
sessionController:      sessionController,
trainingPlanController: trainingPlanController,
```

- [ ] **Step 2: Agregar las 18 rutas en `url_mappings.go`**

Ubicar el bloque de rutas de join-requests (`grep -n "join-requests" cmd/api/app/url_mappings.go`) y agregar después, detrás de `AuthMiddleware()` (mismo grupo que las demás rutas autenticadas):
```go
// Exercise catalog
r.POST("/api/v1/exercises", app.exerciseController.Create)
r.GET("/api/v1/exercises", app.exerciseController.List)
r.GET("/api/v1/exercises/:id", app.exerciseController.Get)
r.PUT("/api/v1/exercises/:id", app.exerciseController.Update)
r.DELETE("/api/v1/exercises/:id", app.exerciseController.Delete)
r.POST("/api/v1/exercises/:id/clone", app.exerciseController.Clone)

// Session catalog
r.POST("/api/v1/sessions", app.sessionController.Create)
r.GET("/api/v1/sessions", app.sessionController.List)
r.GET("/api/v1/sessions/:id", app.sessionController.Get)
r.PUT("/api/v1/sessions/:id", app.sessionController.Update)
r.DELETE("/api/v1/sessions/:id", app.sessionController.Delete)
r.POST("/api/v1/sessions/:id/clone", app.sessionController.Clone)

// TrainingPlan catalog
r.POST("/api/v1/training-plans", app.trainingPlanController.Create)
r.GET("/api/v1/training-plans", app.trainingPlanController.List)
r.GET("/api/v1/training-plans/:id", app.trainingPlanController.Get)
r.PUT("/api/v1/training-plans/:id", app.trainingPlanController.Update)
r.DELETE("/api/v1/training-plans/:id", app.trainingPlanController.Delete)
r.POST("/api/v1/training-plans/:id/clone", app.trainingPlanController.Clone)
```

- [ ] **Step 3: Verificar que compila y arranca**

Run: `go build ./... && go vet ./...`
Expected: verde. Adicionalmente, correr `go test ./cmd/api/app/... -run TestPingRouteExists -v -count=1` y confirmar que sigue en verde (no rompió el wiring existente).

- [ ] **Step 4: Commit**

```bash
git add cmd/api/app/app.go cmd/api/app/url_mappings.go
git commit -m "feat(catalog): wire Exercise/Session/TrainingPlan controllers and routes"
```

---

### Task 13: Swagger y documentación

**Files:**
- Modify: `cmd/api/docs/docs.go`, `cmd/api/docs/swagger.json`, `cmd/api/docs/swagger.yaml` (regenerados)
- Modify: `README.md`

**Interfaces:**
- Consumes: comentarios `godoc` — **agregar** anotaciones `@Summary`/`@Tags`/`@Param`/`@Success`/`@Failure`/`@Router` encima de cada handler de Tasks 9-11 antes de regenerar (mismo formato que `join_request_controller.go`, sección `Swagger` de `.agentics/CONVENTIONS.md`). Como los handlers de este change usan `{"message":"..."}` en vez de `apierror.APIError`, las anotaciones `@Failure` deben apuntar a un tipo `object` genérico o documentarse como texto libre — revisar cómo `swag` maneja esto sin un DTO de error dedicado (opción simple: omitir `@Failure` con tipo y dejar solo el código, `@Failure 404`).

- [ ] **Step 1: Agregar anotaciones godoc a los 18 handlers**

Ejemplo para `ExerciseController.Create` (mismo criterio para los 17 restantes, ajustando summary/params/tags/router):
```go
// Create godoc
// @Summary      Crear ejercicio
// @Tags         exercises
// @Accept       json
// @Produce      json
// @Param        body  body  exercise.ExerciseRequest  true  "Datos del ejercicio"
// @Success      201  {object}  exercise.ExerciseResponse
// @Failure      400
// @Failure      403
// @Router       /api/v1/exercises [post]
func (ec *exerciseController) Create(c *gin.Context) {
```

- [ ] **Step 2: Regenerar swagger**

Run: `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`
Expected: regenera sin errores, diff acotado a los endpoints nuevos.

- [ ] **Step 3: Actualizar tabla de endpoints en `README.md`**

Ubicar la tabla de endpoints existente (buscar la sección con las rutas de `join-requests` ya documentadas) y agregar las 18 rutas nuevas siguiendo el mismo formato de fila.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/controllers/exercise_controller.go cmd/api/controllers/session_controller.go cmd/api/controllers/training_plan_controller.go cmd/api/docs README.md
git commit -m "docs(catalog): add swagger annotations and README endpoint table"
```

---

### Task 14: Verificación final

**Files:** ninguno nuevo — solo verificación.

- [ ] **Step 1: Suite completa sin DB**

Run: `go build ./... && go vet ./... && go test ./... -count=1`
Expected: todo verde.

- [ ] **Step 2: Suite completa con Postgres real**

Run:
```bash
make test-db-up
export TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test
go test ./... -count=1
make test-db-down
```
Expected: todo verde, incluidos los DAOs de este change.

- [ ] **Step 3: Coverage**

Run: `make test-db-up && make coverage-with-db && make test-db-down`
Expected: total del proyecto sigue `>= 80%` (gate de `.testcoverage.yml`). Si baja del 80%, agregar los tests que falten en los archivos de este change antes de continuar — no tocar `.testcoverage.yml`.

- [ ] **Step 4: Prueba manual end-to-end contra testing**

Con el server local corriendo (`go run cmd/api/main.go`, apunta a testing por default): crear un ejercicio, crear una sesión con ese ejercicio en los 3 roles, crear un plan de 2 días (uno `training` con esa sesión, uno `rest`), clonar cada una de las 3 entidades, confirmar que el clon no afecta al original.

- [ ] **Step 5: Commit final si hubo ajustes de cobertura**

```bash
git add -A
git commit -m "test(catalog): close coverage gaps found in final verification"
```

(Si no hubo ajustes, este commit no aplica — no crear un commit vacío.)
