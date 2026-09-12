# Calendario y asignación de planes a grupos — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar el calendario de grupo (`GroupCalendarDay`) — CRUD por día, estampado de planes, operaciones en lote, vistas del corredor, y el clonado por divergencia al editar una sesión asignada — cerrando la segunda mitad del Gap 4.

**Architecture:** Controllers → Services → DAOs, sin delegate. `CalendarService` es nuevo; `SessionService` y `TrainingPlanService` (del change `catalogo-planes-entrenamiento`, ya mergeado en esta misma rama) se **extienden** — sus constructores ganan un dependency nuevo cada uno (`groupCalendarDayDao`, y `SessionService` además `db *gorm.DB` para la transacción de divergencia). Cada task que cambia una firma de constructor existente corrige en el mismo commit el único call site en `app.go` que rompería, para que el build quede verde al final de cada task — la task de wiring final (Task 12) solo agrega piezas nuevas, no repara las ya tocadas.

**Tech Stack:** Go 1.26, Gin, GORM/PostgreSQL, testify.

**Spec:** `openspec/changes/calendario-asignacion-grupos/{proposal.md,design.md,tasks.md,specs/}` + `paceron-frontend/docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` (fuente original). Asume el modelo de `catalogo-planes-entrenamiento` ya implementado (`Session`, `TrainingPlan`, `PlanDay`, `trainingplan.Location`).

## Global Constraints

- Español en comentarios/mensajes de error.
- Error shape `{"message":"..."}` vía `respondCatalogError` (ya existe en `cmd/api/controllers/catalog_common.go`, reusar, no redeclarar).
- Sin FKs físicas en este repo — todo borrado/actualización en cascada se hace a mano, en transacciones GORM explícitas cuando toca a 2+ tablas.
- Patrón de transacción con DAOs recién construidos sobre `tx` (ver `cmd/api/services/team_membership_gate.go`): `db.Transaction(func(tx *gorm.DB) error { txDao := daos.NewXDao(tx); ... })` — nunca pasar `tx` como parámetro a un método de DAO existente (los DAOs de este repo no lo soportan).
- Enums como `string` + `constants` package (`GetValid...()`/`IsValid...()`), igual que el change anterior.
- Ningún delegate — un controller puede depender de 2 services simples cuando solo hace 1 llamada directa cada uno (ej. `SessionController.AssignedGroups` llama a `calendarService.AssignedGroups`, sin componer lógica).
- `go build ./... && go test ./...` (sin DB) deben quedar verdes después de CADA task, incluidas las que cambian firmas de constructores existentes.

---

### Task 1: Modelo GroupCalendarDay + constants + migración

**Files:**
- Create: `cmd/api/domains/dbs/group_calendar_day.go`
- Create: `cmd/api/domains/constants/group_calendar_day_kind.go`
- Modify: `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate)
- Test: `cmd/api/domains/constants/group_calendar_day_kind_test.go`

**Interfaces:**
- Produces: `dbs.GroupCalendarDay` (campos exactos abajo), `constants.GroupCalendarDayKind`/`GetValidGroupCalendarDayKinds()`/`IsValidGroupCalendarDayKind(string) bool`.

- [ ] **Step 1: Modelo**

`cmd/api/domains/dbs/group_calendar_day.go`:
```go
package dbs

import "time"

// GroupCalendarDay es una fila dispersa del calendario real de un grupo —
// sin fila para una fecha significa día vacío. UNIQUE(group_id, date) se
// aplica vía índice compuesto (Step siguiente), no acá.
type GroupCalendarDay struct {
	ID                  int64      `gorm:"column:id;primaryKey"`
	GroupID             int64      `gorm:"column:group_id;not null;uniqueIndex:idx_group_calendar_day_group_date"`
	Date                time.Time  `gorm:"column:date;type:date;not null;uniqueIndex:idx_group_calendar_day_group_date"`
	Kind                string     `gorm:"column:kind;not null"`
	OtherName           *string    `gorm:"column:other_name"`
	SessionID           *int64     `gorm:"column:session_id"`
	CancelledReason     *string    `gorm:"column:cancelled_reason"`
	IsPresencial        bool       `gorm:"column:is_presencial;not null;default:false"`
	PresencialTime      *time.Time `gorm:"column:presencial_time;type:time"`
	PresencialLocation  *string    `gorm:"column:presencial_location;type:jsonb"`
	SourcePlanID        *int64     `gorm:"column:source_plan_id"`
	CreatedAt           time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (GroupCalendarDay) TableName() string { return "group_calendar_days" }
```

- [ ] **Step 2: Constants**

`cmd/api/domains/constants/group_calendar_day_kind.go`:
```go
package constants

// GroupCalendarDayKind extiende PlanDayKind con un 4to valor, cancelled,
// que no tiene sentido en un template (solo existe en el calendario real).
type GroupCalendarDayKind string

const (
	GroupCalendarDayKindRest      GroupCalendarDayKind = "rest"
	GroupCalendarDayKindOther     GroupCalendarDayKind = "other"
	GroupCalendarDayKindTraining  GroupCalendarDayKind = "training"
	GroupCalendarDayKindCancelled GroupCalendarDayKind = "cancelled"
)

func GetValidGroupCalendarDayKinds() []string {
	return []string{
		string(GroupCalendarDayKindRest),
		string(GroupCalendarDayKindOther),
		string(GroupCalendarDayKindTraining),
		string(GroupCalendarDayKindCancelled),
	}
}

func IsValidGroupCalendarDayKind(kind string) bool {
	for _, k := range GetValidGroupCalendarDayKinds() {
		if k == kind {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Test**

`cmd/api/domains/constants/group_calendar_day_kind_test.go`:
```go
package constants

import "testing"

func TestIsValidGroupCalendarDayKind(t *testing.T) {
	if !IsValidGroupCalendarDayKind("cancelled") {
		t.Error("cancelled debería ser válido")
	}
	if IsValidGroupCalendarDayKind("flying") {
		t.Error("flying no debería ser válido")
	}
}

func TestGetValidGroupCalendarDayKinds(t *testing.T) {
	kinds := GetValidGroupCalendarDayKinds()
	if len(kinds) != 4 {
		t.Errorf("esperaba 4 kinds, obtuve %d", len(kinds))
	}
}
```

- [ ] **Step 4: AutoMigrate**

Agregar `&dbs.GroupCalendarDay{}` a la lista de `AutoMigrate(...)` en `cmd/api/infrastructure/postgresdb/postgres.go` (mismo patrón que `&dbs.Exercise{}` etc. del change anterior).

- [ ] **Step 5: Verificar y commitear**

Run: `go build ./... && go vet ./... && go test ./cmd/api/domains/constants/... -run TestGroupCalendarDayKind -v -count=1`
Expected: verde.

```bash
git add cmd/api/domains/dbs/group_calendar_day.go cmd/api/domains/constants/group_calendar_day_kind.go cmd/api/domains/constants/group_calendar_day_kind_test.go cmd/api/infrastructure/postgresdb/postgres.go
git commit -m "feat(calendar): add GroupCalendarDay model and kind enum"
```

---

### Task 2: DTOs de calendario

**Files:**
- Create: `cmd/api/domains/calendar/calendar_day_request.go`
- Create: `cmd/api/domains/calendar/calendar_day_response.go`
- Create: `cmd/api/domains/calendar/next_session_response.go`
- Create: `cmd/api/domains/calendar/calendar_summary_response.go`
- Create: `cmd/api/domains/calendar/bulk_request.go`

**Interfaces:**
- Consumes: `trainingplan.Location` (change anterior, `cmd/api/domains/trainingplan/location.go`).
- Produces: `calendar.CalendarDayRequest`, `calendar.CalendarDayResponse`, `calendar.NextSessionResponse`, `calendar.CalendarSummaryItem`, `calendar.StampRequest`, `calendar.BulkRequest`, `calendar.BulkClearRequest`, `calendar.ShiftRequest` — usados por Tasks 4-11.

- [ ] **Step 1: Día individual (PUT body + response)**

`cmd/api/domains/calendar/calendar_day_request.go`:
```go
package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// CalendarDayRequest es el body de PUT /groups/{id}/calendar/{date} — upsert
// de un día individual. PresencialTime viaja como "HH:MM".
type CalendarDayRequest struct {
	Kind               string               `json:"kind" binding:"required"`
	OtherName          *string              `json:"other_name"`
	SessionID          *int64               `json:"session_id"`
	CancelledReason    *string              `json:"cancelled_reason"`
	IsPresencial       *bool                `json:"is_presencial"`
	PresencialTime     *string              `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}

// StampRequest es el body de POST /groups/{id}/calendar/stamp.
type StampRequest struct {
	PlanID    int64  `json:"plan_id" binding:"required"`
	StartDate string `json:"start_date" binding:"required"` // "YYYY-MM-DD"
	Force     bool   `json:"force"`
}
```

`cmd/api/domains/calendar/bulk_request.go`:
```go
package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// BulkRequest es el body de POST /groups/{id}/calendar/bulk — mismo
// contenido a todas las fechas listadas.
type BulkRequest struct {
	Dates              []string               `json:"dates" binding:"required"`
	Kind               string                 `json:"kind" binding:"required"`
	SessionID          *int64                 `json:"session_id"`
	OtherName          *string                `json:"other_name"`
	IsPresencial       *bool                  `json:"is_presencial"`
	PresencialTime     *string                `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}

// BulkClearRequest es el body de POST /groups/{id}/calendar/bulk-clear.
type BulkClearRequest struct {
	Dates []string `json:"dates" binding:"required"`
}

// ShiftRequest es el body de POST /groups/{id}/calendar/shift.
type ShiftRequest struct {
	FromDate string `json:"from_date" binding:"required"` // "YYYY-MM-DD"
	Days     int    `json:"days" binding:"required"`
}
```

- [ ] **Step 2: Response de día**

`cmd/api/domains/calendar/calendar_day_response.go`:
```go
package calendar

import (
	"time"

	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type CalendarDayResponse struct {
	ID                 int64                  `json:"id"`
	GroupID            int64                  `json:"group_id"`
	Date               string                 `json:"date"` // "YYYY-MM-DD"
	Kind               string                 `json:"kind"`
	OtherName          *string                `json:"other_name"`
	SessionID          *int64                 `json:"session_id"`
	CancelledReason    *string                `json:"cancelled_reason"`
	IsPresencial       bool                   `json:"is_presencial"`
	PresencialTime     *string                `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
	SourcePlanID       *int64                 `json:"source_plan_id"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}
```

- [ ] **Step 3: Vistas del corredor**

`cmd/api/domains/calendar/next_session_response.go`:
```go
package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

type NextSessionResponse struct {
	GroupID            int64                  `json:"group_id"`
	Date               string                 `json:"date"`
	SessionID          *int64                 `json:"session_id"`
	IsPresencial       bool                   `json:"is_presencial"`
	PresencialTime     *string                `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}
```

`cmd/api/domains/calendar/calendar_summary_response.go`:
```go
package calendar

// CalendarSummaryItem es un ítem de GET /users/{id}/calendar-summary y
// también de GET /sessions/{id}/assigned-groups (mismo shape, {group_id,group_name}).
type CalendarSummaryItem struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}
```

- [ ] **Step 4: Verificar y commitear**

Run: `go build ./...`

```bash
git add cmd/api/domains/calendar
git commit -m "feat(calendar): add calendar DTOs"
```

---

### Task 3: GroupCalendarDayDao

**Files:**
- Create: `cmd/api/daos/group_calendar_day_dao.go`
- Test: `cmd/api/daos/group_calendar_day_dao_test.go`

**Interfaces:**
- Consumes: `dbs.GroupCalendarDay` (Task 1).
- Produces: `GroupCalendarDaoInterface{Upsert,FindByGroupAndDate,FindByGroupAndRange,Delete,DeleteByDates,FindNextSessionForGroups,FindDistinctGroupsBySession,ClearSourcePlan,RepointSessionForGroups,UpdateDatesForShift}` — usado por `CalendarService` (Tasks 4-7) y `SessionService`/`TrainingPlanService` extendidos (Tasks 8-9).

- [ ] **Step 1: Escribir el test**

`cmd/api/daos/group_calendar_day_dao_test.go`:
```go
package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestGroupCalendarDayDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupCalendarDayDao(&gorm.DB{})
	var iface GroupCalendarDaoInterface = dao
	_ = iface
}

// setupCalendarGroup crea un owner + team + group real para los tests de
// calendario (no depende de team_dao_test.go, pero reusa persistUser).
func setupCalendarGroup(t *testing.T, db *gorm.DB, emailSuffix string) *dbs.Group {
	t.Helper()
	owner := persistUser(db, "cal-owner-"+emailSuffix+"@test.com", "70000"+emailSuffix)
	team := testTeam(db, "equipo_calendario_"+emailSuffix, owner.ID)
	group := &dbs.Group{Name: "grupo_calendario_" + emailSuffix, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return group
}

func TestGroupCalendarDayDao_UpsertAndFindByGroupAndDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "1")
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"})

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "rest", found.Kind)
}

func TestGroupCalendarDayDao_Upsert_ReplacesExistingDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "2")
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"}))

	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "other", OtherName: strPtrCal("Elongación")})

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Equal(t, "other", found.Kind)
	require.NotNil(t, found.OtherName)
	assert.Equal(t, "Elongación", *found.OtherName)
}

func TestGroupCalendarDayDao_FindByGroupAndRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "3")
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Kind: "rest"}))

	results, err := dao.FindByGroupAndRange(nil, group.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGroupCalendarDayDao_Delete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "4")
	date := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"}))

	err := dao.Delete(nil, group.ID, date)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}

func TestGroupCalendarDayDao_DeleteByDates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "5")
	d1 := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d2, Kind: "rest"}))

	err := dao.DeleteByDates(nil, group.ID, []time.Time{d1, d2})

	require.NoError(t, err)
	results, findErr := dao.FindByGroupAndRange(nil, group.ID, d1, d2)
	require.NoError(t, findErr)
	assert.Empty(t, results)
}

func TestGroupCalendarDayDao_FindNextSessionForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "6")
	past := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	future := time.Now().AddDate(0, 0, 3).Truncate(24 * time.Hour)
	sessionID := int64(1)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "training", SessionID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionID: &sessionID}))

	found, err := dao.FindNextSessionForGroups(nil, []int64{group.ID}, time.Now().Truncate(24*time.Hour))

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(future))
}

func TestGroupCalendarDayDao_FindDistinctGroupsBySession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group1 := setupCalendarGroup(t, db, "7")
	group2 := setupCalendarGroup(t, db, "8")
	sessionID := int64(42)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group1.ID, Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Kind: "training", SessionID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group1.ID, Date: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Kind: "training", SessionID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group2.ID, Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Kind: "training", SessionID: &sessionID}))

	groupIDs, err := dao.FindDistinctGroupsBySession(nil, sessionID)

	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{group1.ID, group2.ID}, groupIDs)
}

func TestGroupCalendarDayDao_ClearSourcePlan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "9")
	planID := int64(99)
	date := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest", SourcePlanID: &planID}))

	err := dao.ClearSourcePlan(nil, planID)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, found.SourcePlanID)
}

func TestGroupCalendarDayDao_RepointSessionForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	groupA := setupCalendarGroup(t, db, "10")
	groupB := setupCalendarGroup(t, db, "11")
	oldSessionID := int64(5)
	newSessionID := int64(6)
	dateA := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)
	dateB := time.Date(2026, 12, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: dateA, Kind: "training", SessionID: &oldSessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: dateB, Kind: "training", SessionID: &oldSessionID}))

	err := dao.RepointSessionForGroups(nil, []int64{groupA.ID}, oldSessionID, newSessionID)

	require.NoError(t, err)
	foundA, _ := dao.FindByGroupAndDate(nil, groupA.ID, dateA)
	require.NotNil(t, foundA.SessionID)
	assert.Equal(t, newSessionID, *foundA.SessionID)
	foundB, _ := dao.FindByGroupAndDate(nil, groupB.ID, dateB)
	require.NotNil(t, foundB.SessionID)
	assert.Equal(t, oldSessionID, *foundB.SessionID, "groupB no estaba en la lista a repuntear, debe quedar intacto")
}

func TestGroupCalendarDayDao_UpdateDatesForShift(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "12")
	original := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: original, Kind: "rest"}))

	err := dao.UpdateDatesForShift(nil, group.ID, original, original.AddDate(0, 0, 3))

	require.NoError(t, err)
	oldFound, _ := dao.FindByGroupAndDate(nil, group.ID, original)
	assert.Nil(t, oldFound)
	newFound, findErr := dao.FindByGroupAndDate(nil, group.ID, original.AddDate(0, 0, 3))
	require.NoError(t, findErr)
	require.NotNil(t, newFound)
}

func strPtrCal(s string) *string { return &s }
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test go test ./cmd/api/daos/... -run TestGroupCalendarDayDao -v -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar el DAO**

`cmd/api/daos/group_calendar_day_dao.go`:
```go
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
		"kind":                day.Kind,
		"other_name":          day.OtherName,
		"session_id":          day.SessionID,
		"cancelled_reason":    day.CancelledReason,
		"is_presencial":       day.IsPresencial,
		"presencial_time":     day.PresencialTime,
		"presencial_location": day.PresencialLocation,
		"source_plan_id":      day.SourcePlanID,
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
	err := d.DB.Model(&dbs.GroupCalendarDay{}).Where("session_id = ?", sessionID).Distinct().Pluck("group_id", &groupIDs).Error
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
		Where("group_id IN ? AND session_id = ?", groupIDs, oldSessionID).
		Update("session_id", newSessionID).Error
}

func (d *groupCalendarDayDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	return d.DB.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", groupID, oldDate).Update("date", newDate).Error
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/daos/group_calendar_day_dao.go cmd/api/daos/group_calendar_day_dao_test.go
git commit -m "feat(calendar): add GroupCalendarDayDao"
```

---

### Task 4: CalendarService — CRUD básico y permisos (GetRange/UpsertDay/DeleteDay)

**Files:**
- Create: `cmd/api/services/calendar_service.go`
- Test: `cmd/api/services/calendar_service_test.go`

**Interfaces:**
- Consumes: `daos.GroupCalendarDaoInterface` (Task 3), `daos.GroupDaoInterface`/`daos.TeamDaoInterface`/`daos.GroupUserDaoInterface` (ya existentes en el repo, del dominio de equipos).
- Produces: `CalendarServiceInterface` (firma completa abajo — Tasks 5-7 la extienden con más métodos en el MISMO archivo, no se redeclara la interfaz, se agregan métodos a ella), sentinels `ErrCalendarGroupNotFound`, `ErrCalendarForbidden`, `ErrCalendarInvalidKind`, `ErrCalendarFieldMismatch`, `ErrCalendarInvalidCancelTransition`.

- [ ] **Step 1: Escribir el test**

`cmd/api/services/calendar_service_test.go`:
```go
package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockGroupCalendarDao struct {
	upsertFn                     func(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	findByGroupAndDateFn         func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	findByGroupAndRangeFn        func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	deleteFn                     func(ctx *gin.Context, groupID int64, date time.Time) error
	deleteByDatesFn              func(ctx *gin.Context, groupID int64, dates []time.Time) error
	findNextSessionForGroupsFn   func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error)
	findDistinctGroupsBySessionFn func(ctx *gin.Context, sessionID int64) ([]int64, error)
	clearSourcePlanFn            func(ctx *gin.Context, planID int64) error
	repointSessionForGroupsFn    func(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error
	updateDatesForShiftFn        func(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
}

func (m *mockGroupCalendarDao) Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, day)
	}
	day.ID = 1
	return nil
}
func (m *mockGroupCalendarDao) FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
	if m.findByGroupAndDateFn != nil {
		return m.findByGroupAndDateFn(ctx, groupID, date)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
	if m.findByGroupAndRangeFn != nil {
		return m.findByGroupAndRangeFn(ctx, groupID, from, to)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) Delete(ctx *gin.Context, groupID int64, date time.Time) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, groupID, date)
	}
	return nil
}
func (m *mockGroupCalendarDao) DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error {
	if m.deleteByDatesFn != nil {
		return m.deleteByDatesFn(ctx, groupID, dates)
	}
	return nil
}
func (m *mockGroupCalendarDao) FindNextSessionForGroups(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
	if m.findNextSessionForGroupsFn != nil {
		return m.findNextSessionForGroupsFn(ctx, groupIDs, fromDate)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) FindDistinctGroupsBySession(ctx *gin.Context, sessionID int64) ([]int64, error) {
	if m.findDistinctGroupsBySessionFn != nil {
		return m.findDistinctGroupsBySessionFn(ctx, sessionID)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	if m.clearSourcePlanFn != nil {
		return m.clearSourcePlanFn(ctx, planID)
	}
	return nil
}
func (m *mockGroupCalendarDao) RepointSessionForGroups(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error {
	if m.repointSessionForGroupsFn != nil {
		return m.repointSessionForGroupsFn(ctx, groupIDs, oldSessionID, newSessionID)
	}
	return nil
}
func (m *mockGroupCalendarDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	if m.updateDatesForShiftFn != nil {
		return m.updateDatesForShiftFn(ctx, groupID, oldDate, newDate)
	}
	return nil
}

type mockGroupDao struct {
	createFn            func(ctx *gin.Context, group *dbs.Group) error
	findByIDFn          func(ctx *gin.Context, id int64) (*dbs.Group, error)
	findByIDAndTeamIDFn func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error)
	getAllFn            func(ctx *gin.Context) ([]dbs.Group, error)
	getByTeamIDFn       func(ctx *gin.Context, teamID int64) ([]dbs.Group, error)
	updateFn            func(ctx *gin.Context, group *dbs.Group) error
	softDeleteFn        func(ctx *gin.Context, id int64) error
	softDeleteByTeamIDFn func(ctx *gin.Context, teamID int64) error
}

func (m *mockGroupDao) Create(ctx *gin.Context, group *dbs.Group) error {
	if m.createFn != nil {
		return m.createFn(ctx, group)
	}
	return nil
}
func (m *mockGroupDao) FindByID(ctx *gin.Context, id int64) (*dbs.Group, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockGroupDao) FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
	if m.findByIDAndTeamIDFn != nil {
		return m.findByIDAndTeamIDFn(ctx, groupID, teamID)
	}
	return nil, nil
}
func (m *mockGroupDao) GetAll(ctx *gin.Context) ([]dbs.Group, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil
}
func (m *mockGroupDao) GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
	if m.getByTeamIDFn != nil {
		return m.getByTeamIDFn(ctx, teamID)
	}
	return nil, nil
}
func (m *mockGroupDao) Update(ctx *gin.Context, group *dbs.Group) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, group)
	}
	return nil
}
func (m *mockGroupDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockGroupDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	if m.softDeleteByTeamIDFn != nil {
		return m.softDeleteByTeamIDFn(ctx, teamID)
	}
	return nil
}

type mockGroupUserDao struct {
	createFn             func(ctx *gin.Context, gu *dbs.GroupUser) error
	findByGroupAndUserFn func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error)
	findByGroupIDFn      func(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error)
	findByUserIDFn       func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	softDeleteFn         func(ctx *gin.Context, id int64) error
	softDeleteByTeamIDFn func(ctx *gin.Context, teamID int64) error
}

func (m *mockGroupUserDao) Create(ctx *gin.Context, gu *dbs.GroupUser) error {
	if m.createFn != nil {
		return m.createFn(ctx, gu)
	}
	return nil
}
func (m *mockGroupUserDao) FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
	if m.findByGroupAndUserFn != nil {
		return m.findByGroupAndUserFn(ctx, groupID, userID)
	}
	return nil, nil
}
func (m *mockGroupUserDao) FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
	if m.findByGroupIDFn != nil {
		return m.findByGroupIDFn(ctx, groupID)
	}
	return nil, nil
}
func (m *mockGroupUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockGroupUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockGroupUserDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	if m.softDeleteByTeamIDFn != nil {
		return m.softDeleteByTeamIDFn(ctx, teamID)
	}
	return nil
}

func TestCalendarService_GetRange_OwnerAllowed(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	calDao := &mockGroupCalendarDao{}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.GetRange(nil, 1, 7, time.Now(), time.Now())

	require.NoError(t, err)
}

func TestCalendarService_GetRange_ForbiddenForOutsider(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	groupUserDao := &mockGroupUserDao{findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) { return nil, nil }}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)

	_, err := svc.GetRange(nil, 1, 99, time.Now(), time.Now())

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_UpsertDay_NonOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 99, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_UpsertDay_OtherRequiresOtherName(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "other"})

	assert.ErrorIs(t, err, ErrCalendarFieldMismatch)
}

func TestCalendarService_UpsertDay_CancelFromRestRejected(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "rest"}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	reason := "lluvia"

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	assert.ErrorIs(t, err, ErrCalendarInvalidCancelTransition)
}

func TestCalendarService_UpsertDay_CancelFromTrainingAccepted(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	sessionID := int64(3)
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", SessionID: &sessionID}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	reason := "lluvia"

	resp, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	require.NoError(t, err)
	assert.Equal(t, "cancelled", resp.Kind)
}

func TestCalendarService_DeleteDay_NonOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 99, time.Now())

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestCalendarService -v -count=1`
Expected: FAIL — `NewCalendarService` no existe.

- [ ] **Step 3: Implementar el service (parte 1 — CRUD básico y permisos)**

`cmd/api/services/calendar_service.go`:
```go
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrCalendarGroupNotFound          = errors.New("grupo no encontrado")
	ErrCalendarForbidden              = errors.New("no autorizado")
	ErrCalendarInvalidKind            = errors.New("kind inválido")
	ErrCalendarFieldMismatch          = errors.New("combinación de campos inválida para el kind del día")
	ErrCalendarInvalidCancelTransition = errors.New("solo se puede cancelar un día que está en training")
	ErrCalendarPlanNotFound           = errors.New("plan no encontrado")
	ErrCalendarPlanForbidden          = errors.New("el plan no pertenece al entrenador dueño del grupo")
	ErrCalendarStampConflict          = errors.New("hay fechas con contenido existente")
	ErrCalendarShiftCollision         = errors.New("el corrimiento haría chocar dos fechas")
	ErrCalendarUserMismatch           = errors.New("no podés consultar los datos de otro usuario")
)

// CalendarServiceInterface se completa en Tasks 5-7 (Stamp, Bulk/BulkClear/
// Shift, NextSession/CalendarSummary/AssignedGroups) — todos métodos del
// mismo archivo/struct, no una interfaz separada.
type CalendarServiceInterface interface {
	GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error)
	UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error)
	DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error
	Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error)
	Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error)
	BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error
	Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error)
	NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error)
	CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error)
	AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error)
}

type calendarService struct {
	calendarDao     daos.GroupCalendarDaoInterface
	groupDao        daos.GroupDaoInterface
	teamDao         daos.TeamDaoInterface
	groupUserDao    daos.GroupUserDaoInterface
	teamUserDao     daos.TeamUserDaoInterface
	trainingPlanDao daos.TrainingPlanDaoInterface
	planDayDao      daos.PlanDayDaoInterface
	sessionDao      daos.SessionDaoInterface
	db              *gorm.DB
}

func NewCalendarService(
	calendarDao daos.GroupCalendarDaoInterface,
	groupDao daos.GroupDaoInterface,
	teamDao daos.TeamDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	trainingPlanDao daos.TrainingPlanDaoInterface,
	planDayDao daos.PlanDayDaoInterface,
	sessionDao daos.SessionDaoInterface,
	db *gorm.DB,
) CalendarServiceInterface {
	return &calendarService{
		calendarDao: calendarDao, groupDao: groupDao, teamDao: teamDao, groupUserDao: groupUserDao,
		teamUserDao: teamUserDao, trainingPlanDao: trainingPlanDao, planDayDao: planDayDao, sessionDao: sessionDao, db: db,
	}
}

// isGroupOwner devuelve nil si callerID es el entrenador dueño del equipo del grupo.
func (s *calendarService) isGroupOwner(ctx *gin.Context, groupID, callerID int64) error {
	group, err := s.groupDao.FindByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error al buscar grupo")
	}
	if group == nil {
		return ErrCalendarGroupNotFound
	}
	team, err := s.teamDao.FindByID(ctx, group.TeamID)
	if err != nil {
		return fmt.Errorf("error al buscar equipo")
	}
	if team == nil || team.OwnerID != callerID {
		return ErrCalendarForbidden
	}
	return nil
}

// isGroupOwnerOrMember devuelve nil si callerID es el dueño o un miembro
// activo del grupo (lectura, spec §4).
func (s *calendarService) isGroupOwnerOrMember(ctx *gin.Context, groupID, callerID int64) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err == nil {
		return nil
	} else if !errors.Is(err, ErrCalendarForbidden) {
		return err
	}
	member, err := s.groupUserDao.FindByGroupAndUser(ctx, groupID, callerID)
	if err != nil {
		return fmt.Errorf("error al validar membresía")
	}
	if member == nil {
		return ErrCalendarForbidden
	}
	return nil
}

func (s *calendarService) validateDayFields(req calendar.CalendarDayRequest, currentKind string) error {
	if !constants.IsValidGroupCalendarDayKind(req.Kind) {
		return ErrCalendarInvalidKind
	}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		if req.OtherName == nil {
			return ErrCalendarFieldMismatch
		}
	case string(constants.GroupCalendarDayKindTraining):
		if req.SessionID == nil {
			return ErrCalendarFieldMismatch
		}
	case string(constants.GroupCalendarDayKindCancelled):
		if req.CancelledReason == nil {
			return ErrCalendarFieldMismatch
		}
		if currentKind != string(constants.GroupCalendarDayKindTraining) {
			return ErrCalendarInvalidCancelTransition
		}
	}
	isPresencial := req.IsPresencial != nil && *req.IsPresencial
	if isPresencial && (req.PresencialTime == nil || req.PresencialLocation == nil) {
		return ErrCalendarFieldMismatch
	}
	return nil
}

func (s *calendarService) buildRow(ctx *gin.Context, groupID int64, date time.Time, req calendar.CalendarDayRequest) (*dbs.GroupCalendarDay, error) {
	row := &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: req.Kind}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		row.OtherName = req.OtherName
	case string(constants.GroupCalendarDayKindTraining):
		row.SessionID = req.SessionID
	case string(constants.GroupCalendarDayKindCancelled):
		row.CancelledReason = req.CancelledReason
	}
	if req.IsPresencial != nil && *req.IsPresencial {
		row.IsPresencial = true
		parsedTime, err := time.Parse("15:04", *req.PresencialTime)
		if err != nil {
			return nil, fmt.Errorf("presencial_time debe tener formato HH:MM")
		}
		row.PresencialTime = &parsedTime
		locationJSON, err := jsonMarshalLocation(req.PresencialLocation)
		if err != nil {
			return nil, err
		}
		row.PresencialLocation = locationJSON
	}
	return row, nil
}

func (s *calendarService) GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwnerOrMember(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	days, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, from, to)
	if err != nil {
		customlogger.Error(ctx, "error listing calendar range", err, customlogger.TagMethod("GetRange"))
		return nil, fmt.Errorf("error al listar calendario")
	}
	responses := make([]calendar.CalendarDayResponse, len(days))
	for i, d := range days {
		responses[i] = toCalendarDayResponse(d)
	}
	return responses, nil
}

func (s *calendarService) UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	existing, err := s.calendarDao.FindByGroupAndDate(ctx, groupID, date)
	if err != nil {
		return nil, fmt.Errorf("error al buscar día de calendario")
	}
	currentKind := ""
	if existing != nil {
		currentKind = existing.Kind
	}
	if err := s.validateDayFields(req, currentKind); err != nil {
		return nil, err
	}
	row, err := s.buildRow(ctx, groupID, date, req)
	if err != nil {
		return nil, err
	}
	if err := s.calendarDao.Upsert(ctx, row); err != nil {
		customlogger.Error(ctx, "error upserting calendar day", err, customlogger.TagMethod("UpsertDay"))
		return nil, fmt.Errorf("error al guardar día de calendario")
	}
	resp := toCalendarDayResponse(*row)
	return &resp, nil
}

func (s *calendarService) DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return err
	}
	if err := s.calendarDao.Delete(ctx, groupID, date); err != nil {
		customlogger.Error(ctx, "error deleting calendar day", err, customlogger.TagMethod("DeleteDay"))
		return fmt.Errorf("error al borrar día de calendario")
	}
	return nil
}
```

**Nota**: los métodos `Stamp`, `Bulk`, `BulkClear`, `Shift`, `NextSession`, `CalendarSummary`, `AssignedGroups`, y los helpers `toCalendarDayResponse`/`jsonMarshalLocation`, se agregan en Tasks 5-7 al MISMO archivo `calendar_service.go` (no crear un archivo nuevo) — sin esos métodos, este archivo no compila todavía porque `CalendarServiceInterface` los exige. Este task deja el archivo con esos métodos ausentes intencionalmente: el build de esta task específica NO estará verde hasta que también se agregue al menos una implementación stub o se ejecuten las Tasks 5-7 en el mismo lote. **Para que esta task sea autocontenida y buildable por sí sola**, agregar también los siguientes 4 métodos con implementación real minimalista (no lógica de negocio compleja, se reemplazan/completan en Tasks 5-7):

```go
func toCalendarDayResponse(d dbs.GroupCalendarDay) calendar.CalendarDayResponse {
	resp := calendar.CalendarDayResponse{
		ID: d.ID, GroupID: d.GroupID, Date: d.Date.Format("2006-01-02"), Kind: d.Kind,
		OtherName: d.OtherName, SessionID: d.SessionID, CancelledReason: d.CancelledReason,
		IsPresencial: d.IsPresencial, SourcePlanID: d.SourcePlanID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.PresencialTime != nil {
		formatted := d.PresencialTime.Format("15:04")
		resp.PresencialTime = &formatted
	}
	if d.PresencialLocation != nil {
		loc, err := jsonUnmarshalLocation(*d.PresencialLocation)
		if err == nil {
			resp.PresencialLocation = loc
		}
	}
	return resp
}
```

Agregar también en este mismo archivo (compartido por Tasks 5-7, no duplicar en otro archivo):
```go
import "encoding/json"
import "simple-arq-golang/cmd/api/domains/trainingplan"

func jsonMarshalLocation(loc *trainingplan.Location) (*string, error) {
	b, err := json.Marshal(loc)
	if err != nil {
		return nil, fmt.Errorf("error al serializar ubicación")
	}
	s := string(b)
	return &s, nil
}

func jsonUnmarshalLocation(s string) (*trainingplan.Location, error) {
	var loc trainingplan.Location
	if err := json.Unmarshal([]byte(s), &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}
```

Y para que `CalendarServiceInterface` compile ya en esta task, agregar placeholders REALES (no vacíos — devuelven `fmt.Errorf("no implementado todavía")`) para `Stamp`, `Bulk`, `BulkClear`, `Shift`, `NextSession`, `CalendarSummary`, `AssignedGroups`, que Tasks 5-7 reemplazan con la lógica real:

```go
func (s *calendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 5")
}
func (s *calendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	return fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
func (s *calendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
func (s *calendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
```

Esto es temporal y deliberado — cada placeholder se reemplaza en su task correspondiente (Tasks 5, 6, 7), nunca se queda así al final del plan.

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS (los 8 tests de esta task, los placeholders de Stamp/Bulk/etc. no se testean acá).

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/calendar_service.go cmd/api/services/calendar_service_test.go
git commit -m "feat(calendar): add CalendarService core CRUD and permission checks"
```

---

### Task 5: CalendarService — Stamp

**Files:**
- Modify: `cmd/api/services/calendar_service.go` (reemplaza el placeholder de `Stamp`)
- Modify: `cmd/api/services/calendar_service_test.go` (agrega tests de `Stamp`)

**Interfaces:**
- Consumes: `daos.TrainingPlanDaoInterface.FindByID` y `daos.PlanDayDaoInterface.FindByPlan` (change anterior, ya inyectados en `calendarService` desde Task 4).

- [ ] **Step 1: Agregar tests de Stamp**

Agregar a `cmd/api/services/calendar_service_test.go` (mismo archivo, junto a los mocks ya definidos — agregar `mockTrainingPlanDao`/`mockPlanDayDao`/`mockSessionDao` como parámetros si no vinieron ya de Task 4's test setup; reusar si el paquete `services` ya los tiene declarados en otro `_test.go` del mismo paquete, como `training_plan_service_test.go` — **no redeclarar los mocks de `mockTrainingPlanDao`/`mockPlanDayDao`/`mockSessionDao`, ya existen en el paquete `services` desde el change anterior**):

```go
func TestCalendarService_Stamp_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	sessionID := int64(3)
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{
			{SequenceNo: 1, Kind: "rest"},
			{SequenceNo: 2, Kind: "training", SessionID: &sessionID},
		}, nil
	}}
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	resp, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestCalendarService_Stamp_PlanFromOtherOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 99}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, &mockPlanDayDao{}, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	assert.ErrorIs(t, err, ErrCalendarPlanForbidden)
}

func TestCalendarService_Stamp_ConflictWithoutForce(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{SequenceNo: 1, Kind: "rest"}}, nil
	}}
	occupiedDate, _ := time.Parse("2006-01-02", "2026-10-01")
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: groupID, Date: occupiedDate, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	assert.ErrorIs(t, err, ErrCalendarStampConflict)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestCalendarService_Stamp -v -count=1`
Expected: FAIL (el placeholder devuelve "no implementado todavía").

- [ ] **Step 3: Implementar `Stamp`**

Reemplazar el placeholder de `Stamp` en `cmd/api/services/calendar_service.go`:
```go
func (s *calendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	plan, err := s.trainingPlanDao.FindByID(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar plan")
	}
	if plan == nil {
		return nil, ErrCalendarPlanNotFound
	}
	if plan.OwnerID != callerID {
		return nil, ErrCalendarPlanForbidden
	}
	planDays, err := s.planDayDao.FindByPlan(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar días del plan")
	}
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("start_date debe tener formato YYYY-MM-DD")
	}

	rows := make([]dbs.GroupCalendarDay, len(planDays))
	targetDates := make([]time.Time, len(planDays))
	for i, pd := range planDays {
		date := startDate.AddDate(0, 0, pd.SequenceNo-1)
		targetDates[i] = date
		row := dbs.GroupCalendarDay{
			GroupID: groupID, Date: date, Kind: pd.Kind, OtherName: pd.OtherName, SessionID: pd.SessionID,
			IsPresencial: pd.DefaultPresencial, PresencialTime: pd.DefaultTime, PresencialLocation: pd.DefaultLocation,
			SourcePlanID: &req.PlanID,
		}
		rows[i] = row
	}

	if !req.Force {
		minDate, maxDate := targetDates[0], targetDates[0]
		for _, d := range targetDates {
			if d.Before(minDate) {
				minDate = d
			}
			if d.After(maxDate) {
				maxDate = d
			}
		}
		existing, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, minDate, maxDate)
		if err != nil {
			return nil, fmt.Errorf("error al validar conflictos")
		}
		occupied := make(map[string]bool, len(existing))
		for _, e := range existing {
			occupied[e.Date.Format("2006-01-02")] = true
		}
		conflict := false
		for _, d := range targetDates {
			if occupied[d.Format("2006-01-02")] {
				conflict = true
				break
			}
		}
		if conflict {
			return nil, ErrCalendarStampConflict
		}
	}

	responses := make([]calendar.CalendarDayResponse, len(rows))
	for i := range rows {
		if err := s.calendarDao.Upsert(ctx, &rows[i]); err != nil {
			customlogger.Error(ctx, "error stamping calendar day", err, customlogger.TagMethod("Stamp"))
			return nil, fmt.Errorf("error al estampar plan")
		}
		responses[i] = toCalendarDayResponse(rows[i])
	}
	return responses, nil
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/calendar_service.go cmd/api/services/calendar_service_test.go
git commit -m "feat(calendar): implement Stamp (copy plan into group calendar)"
```

---

### Task 6: CalendarService — Bulk / BulkClear / Shift

**Files:**
- Modify: `cmd/api/services/calendar_service.go` (reemplaza los placeholders de `Bulk`/`BulkClear`/`Shift`)
- Modify: `cmd/api/services/calendar_service_test.go`

- [ ] **Step 1: Agregar tests**

Agregar a `cmd/api/services/calendar_service_test.go`:
```go
func TestCalendarService_Bulk_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"2026-10-01", "2026-10-02"}, Kind: "rest"})

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestCalendarService_BulkClear_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	deleted := false
	calDao := &mockGroupCalendarDao{deleteByDatesFn: func(ctx *gin.Context, groupID int64, dates []time.Time) error {
		deleted = true
		return nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{"2026-10-01"}})

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestCalendarService_Shift_NoCollision(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	fromDate, _ := time.Parse("2006-01-02", "2026-10-01")
	calDao := &mockGroupCalendarDao{
		findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
			return []dbs.GroupCalendarDay{{GroupID: groupID, Date: fromDate, Kind: "rest"}}, nil
		},
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "2026-10-01", Days: 2})

	require.NoError(t, err)
	assert.Len(t, resp, 1)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run "TestCalendarService_Bulk|TestCalendarService_Shift" -v -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar `Bulk`/`BulkClear`/`Shift`**

Reemplazar los 3 placeholders en `cmd/api/services/calendar_service.go`:
```go
func (s *calendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	dayReq := calendar.CalendarDayRequest{
		Kind: req.Kind, SessionID: req.SessionID, OtherName: req.OtherName,
		IsPresencial: req.IsPresencial, PresencialTime: req.PresencialTime, PresencialLocation: req.PresencialLocation,
	}
	if err := s.validateDayFields(dayReq, ""); err != nil {
		return nil, err
	}
	responses := make([]calendar.CalendarDayResponse, 0, len(req.Dates))
	for _, dateStr := range req.Dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("fecha inválida en dates: %s", dateStr)
		}
		row, err := s.buildRow(ctx, groupID, date, dayReq)
		if err != nil {
			return nil, err
		}
		if err := s.calendarDao.Upsert(ctx, row); err != nil {
			customlogger.Error(ctx, "error bulk-upserting calendar day", err, customlogger.TagMethod("Bulk"))
			return nil, fmt.Errorf("error al aplicar bulk")
		}
		responses = append(responses, toCalendarDayResponse(*row))
	}
	return responses, nil
}

func (s *calendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return err
	}
	dates := make([]time.Time, 0, len(req.Dates))
	for _, dateStr := range req.Dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return fmt.Errorf("fecha inválida en dates: %s", dateStr)
		}
		dates = append(dates, date)
	}
	if err := s.calendarDao.DeleteByDates(ctx, groupID, dates); err != nil {
		customlogger.Error(ctx, "error bulk-clearing calendar days", err, customlogger.TagMethod("BulkClear"))
		return fmt.Errorf("error al limpiar fechas")
	}
	return nil
}

func (s *calendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	fromDate, err := time.Parse("2006-01-02", req.FromDate)
	if err != nil {
		return nil, fmt.Errorf("from_date debe tener formato YYYY-MM-DD")
	}
	if req.Days <= 0 {
		return nil, fmt.Errorf("days debe ser un entero positivo")
	}
	farFuture := fromDate.AddDate(1, 0, 0)
	affected, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, fromDate, farFuture)
	if err != nil {
		return nil, fmt.Errorf("error al buscar filas a correr")
	}
	before, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, fromDate.AddDate(0, 0, -365), fromDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, fmt.Errorf("error al validar colisiones")
	}
	unaffectedDates := make(map[string]bool, len(before))
	for _, b := range before {
		unaffectedDates[b.Date.Format("2006-01-02")] = true
	}
	for _, a := range affected {
		newDate := a.Date.AddDate(0, 0, req.Days)
		if unaffectedDates[newDate.Format("2006-01-02")] {
			return nil, ErrCalendarShiftCollision
		}
	}
	responses := make([]calendar.CalendarDayResponse, len(affected))
	for i, a := range affected {
		newDate := a.Date.AddDate(0, 0, req.Days)
		if err := s.calendarDao.UpdateDatesForShift(ctx, groupID, a.Date, newDate); err != nil {
			customlogger.Error(ctx, "error shifting calendar day", err, customlogger.TagMethod("Shift"))
			return nil, fmt.Errorf("error al correr fechas")
		}
		a.Date = newDate
		responses[i] = toCalendarDayResponse(a)
	}
	return responses, nil
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/calendar_service.go cmd/api/services/calendar_service_test.go
git commit -m "feat(calendar): implement Bulk, BulkClear and Shift"
```

---

### Task 7: CalendarService — NextSession / CalendarSummary / AssignedGroups

**Files:**
- Modify: `cmd/api/services/calendar_service.go` (reemplaza los 3 placeholders restantes)
- Modify: `cmd/api/services/calendar_service_test.go`

- [ ] **Step 1: Agregar tests**

```go
func TestCalendarService_NextSession_Found(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	sessionID := int64(3)
	nextDate, _ := time.Parse("2006-01-02", "2026-10-10")
	calDao := &mockGroupCalendarDao{findNextSessionForGroupsFn: func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "training", SessionID: &sessionID}, nil
	}}
	svc := NewCalendarService(calDao, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.GroupID)
}

func TestCalendarService_NextSession_NoneReturnsNil(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	assert.Nil(t, resp)
}

func TestCalendarService_CalendarSummary_ListsGroups(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}, {GroupID: 2, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, Name: fmt.Sprintf("Grupo %d", id)}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.CalendarSummary(nil, 42)

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestCalendarService_AssignedGroups_Distinct(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, Name: fmt.Sprintf("Grupo %d", id)}, nil
	}}
	calDao := &mockGroupCalendarDao{findDistinctGroupsBySessionFn: func(ctx *gin.Context, sessionID int64) ([]int64, error) {
		return []int64{1, 2}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.AssignedGroups(nil, 99)

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}
```

Agregar `"fmt"` al import de test si no está (ya debería estarlo por otros archivos del paquete, verificar).

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run "TestCalendarService_NextSession|TestCalendarService_CalendarSummary|TestCalendarService_AssignedGroups" -v -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar los 3 métodos**

Reemplazar los 3 placeholders restantes en `cmd/api/services/calendar_service.go`:
```go
func (s *calendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	memberships, err := s.groupUserDao.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar grupos del usuario")
	}
	groupIDs := make([]int64, len(memberships))
	for i, m := range memberships {
		groupIDs[i] = m.GroupID
	}
	day, err := s.calendarDao.FindNextSessionForGroups(ctx, groupIDs, time.Now().Truncate(24*time.Hour))
	if err != nil {
		customlogger.Error(ctx, "error finding next session", err, customlogger.TagMethod("NextSession"))
		return nil, fmt.Errorf("error al buscar próxima sesión")
	}
	if day == nil {
		return nil, nil
	}
	resp := &calendar.NextSessionResponse{
		GroupID: day.GroupID, Date: day.Date.Format("2006-01-02"), SessionID: day.SessionID, IsPresencial: day.IsPresencial,
	}
	if day.PresencialTime != nil {
		formatted := day.PresencialTime.Format("15:04")
		resp.PresencialTime = &formatted
	}
	if day.PresencialLocation != nil {
		loc, err := jsonUnmarshalLocation(*day.PresencialLocation)
		if err == nil {
			resp.PresencialLocation = loc
		}
	}
	return resp, nil
}

func (s *calendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	memberships, err := s.groupUserDao.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar grupos del usuario")
	}
	items := make([]calendar.CalendarSummaryItem, 0, len(memberships))
	for _, m := range memberships {
		group, err := s.groupDao.FindByID(ctx, m.GroupID)
		if err != nil || group == nil {
			continue
		}
		items = append(items, calendar.CalendarSummaryItem{GroupID: group.ID, GroupName: group.Name})
	}
	return items, nil
}

func (s *calendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	groupIDs, err := s.calendarDao.FindDistinctGroupsBySession(ctx, sessionID)
	if err != nil {
		customlogger.Error(ctx, "error finding assigned groups", err, customlogger.TagMethod("AssignedGroups"))
		return nil, fmt.Errorf("error al buscar grupos asignados")
	}
	items := make([]calendar.CalendarSummaryItem, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, err := s.groupDao.FindByID(ctx, id)
		if err != nil || group == nil {
			continue
		}
		items = append(items, calendar.CalendarSummaryItem{GroupID: group.ID, GroupName: group.Name})
	}
	return items, nil
}
```

- [ ] **Step 4: Correr el test, verificar que pasa**

Run: mismo comando del Step 2, y adicionalmente `go test ./cmd/api/services/... -run TestCalendarService -v -count=1` para confirmar TODOS los tests de `calendar_service_test.go` (Tasks 4-7) siguen en verde juntos.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/services/calendar_service.go cmd/api/services/calendar_service_test.go
git commit -m "feat(calendar): implement NextSession, CalendarSummary and AssignedGroups"
```

---

### Task 8: Clonado por divergencia — extender SessionService

**Files:**
- Modify: `cmd/api/domains/session/session_request.go` (+3 campos opcionales)
- Modify: `cmd/api/services/session_service.go` (constructor +2 args, refactor `cloneInternal`, `Update` con divergencia)
- Modify: `cmd/api/services/session_service_test.go` (actualizar TODAS las llamadas existentes a `NewSessionService`, agregar tests nuevos)
- Modify: `cmd/api/app/app.go` (única línea existente que llama a `NewSessionService`, agregar los 2 args nuevos)

**Interfaces:**
- Consumes: `daos.GroupCalendarDaoInterface` (Task 3).
- Produces: `NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, groupCalendarDayDao, db)` — firma nueva de 5 args (antes 3). **Cualquier otro archivo que instancie `SessionService` debe actualizarse en esta misma task** (grep `NewSessionService(` en todo el repo antes de terminar).

- [ ] **Step 1: Extender `SessionRequest`**

En `cmd/api/domains/session/session_request.go`, agregar al final del struct `SessionRequest`:
```go
	// Campos del flujo de clonado por divergencia (calendario-asignacion-grupos,
	// solo se usan en PUT, ignorados en POST).
	ExcludeGroupIDs  *[]int64 `json:"exclude_group_ids"`
	CloneName        *string  `json:"clone_name"`
	CloneDescription *string  `json:"clone_description"`
```

- [ ] **Step 2: Escribir/actualizar tests de `session_service_test.go`**

Primero, correr `grep -n "NewSessionService(" cmd/api/services/session_service_test.go` — cada ocurrencia con 3 argumentos pasa a 5, agregando `&mockGroupCalendarDao{}` y `nil` (sin transacción real en tests unitarios, el `db` solo se usa cuando `ExcludeGroupIDs` no es nil/vacío):

Reemplazar cada `NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)` (o variantes con `&mockSessionDao{}` etc.) por `NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, &mockGroupCalendarDao{}, nil)` — usar el MISMO `mockGroupCalendarDao` de `calendar_service_test.go` (mismo paquete `services`, no redeclarar).

Agregar el test nuevo específico de divergencia — test de integración con Postgres real (mismo criterio que `payment_service_test.go`/`user_role_service_test.go`, que ya usan `testutils.SetupTestDB(t)` en el paquete `services` cuando la lógica bajo prueba compone una transacción real): la divergencia corre dentro de `s.db.Transaction(...)`, construyendo DAOs frescos sobre `tx` — no es mockeable de forma significativa sin perder la garantía de atomicidad que el test quiere probar. Agregar `"simple-arq-golang/cmd/api/testutils"` y `"time"` al import del archivo si no están.

```go
func TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionDao(db)
	sessionExerciseDao := NewSessionExerciseDao(db)
	exerciseDao := NewExerciseDao(db)
	calendarDao := NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-divergence-owner@test.com", DNI: "50000090", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)

	warmup := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote", Kind: "jogging"}
	require.NoError(t, db.Create(warmup).Error)
	main := &dbs.Exercise{OwnerID: owner.ID, Name: "Serie", Kind: "running"}
	require.NoError(t, db.Create(main).Error)
	cooldown := &dbs.Exercise{OwnerID: owner.ID, Name: "Elongación", Kind: "elongation"}
	require.NoError(t, db.Create(cooldown).Error)

	original := &dbs.Session{OwnerID: owner.ID, Name: "Sesión original"}
	require.NoError(t, sessionDao.Create(nil, original))
	require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, original.ID, []dbs.SessionExercise{
		{ExerciseID: warmup.ID, Role: "warmup", RepeatCount: 1, RestMinutes: 0},
		{ExerciseID: main.ID, Role: "main", RepeatCount: 3, RestMinutes: 2},
		{ExerciseID: cooldown.ID, Role: "cooldown", RepeatCount: 1, RestMinutes: 0},
	}))

	team := &dbs.Team{Name: "Equipo divergencia", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	excludedGroup := &dbs.Group{Name: "Grupo excluido", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(excludedGroup).Error)
	keptGroup := &dbs.Group{Name: "Grupo que se queda", TeamID: team.ID, IsMain: false}
	require.NoError(t, db.Create(keptGroup).Error)

	date := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: excludedGroup.ID, Date: date, Kind: "training", SessionID: &original.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: keptGroup.ID, Date: date, Kind: "training", SessionID: &original.ID}))

	excludeIDs := []int64{excludedGroup.ID}
	newName := "Sesión editada"
	resp, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: newName, ExcludeGroupIDs: &excludeIDs,
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, newName, resp.Name)

	excludedDay, err := calendarDao.FindByGroupAndDate(nil, excludedGroup.ID, date)
	require.NoError(t, err)
	require.NotNil(t, excludedDay.SessionID)
	assert.NotEqual(t, original.ID, *excludedDay.SessionID, "el grupo excluido debe apuntar al clon, no a la sesión original")

	clonedSessionID := *excludedDay.SessionID
	clonedExercises, err := sessionExerciseDao.FindBySession(nil, clonedSessionID)
	require.NoError(t, err)
	assert.Len(t, clonedExercises, 3, "el clon debe tener copia profunda de los 3 ejercicios")

	keptDay, err := calendarDao.FindByGroupAndDate(nil, keptGroup.ID, date)
	require.NoError(t, err)
	require.NotNil(t, keptDay.SessionID)
	assert.Equal(t, original.ID, *keptDay.SessionID, "el grupo no excluido debe seguir apuntando a la sesión original")

	updatedOriginal, err := sessionDao.FindByID(nil, original.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updatedOriginal.Name)
}
```

- [ ] **Step 3: Refactor `Clone` en `cloneInternal` + extender `Update`**

En `cmd/api/services/session_service.go`:

1. Cambiar el struct y el constructor:
```go
type sessionService struct {
	sessionDao          daos.SessionDaoInterface
	sessionExerciseDao  daos.SessionExerciseDaoInterface
	exerciseDao         daos.ExerciseDaoInterface
	groupCalendarDayDao daos.GroupCalendarDaoInterface
	db                  *gorm.DB
}

func NewSessionService(
	sessionDao daos.SessionDaoInterface,
	sessionExerciseDao daos.SessionExerciseDaoInterface,
	exerciseDao daos.ExerciseDaoInterface,
	groupCalendarDayDao daos.GroupCalendarDaoInterface,
	db *gorm.DB,
) SessionServiceInterface {
	return &sessionService{
		sessionDao: sessionDao, sessionExerciseDao: sessionExerciseDao, exerciseDao: exerciseDao,
		groupCalendarDayDao: groupCalendarDayDao, db: db,
	}
}
```
Agregar `"gorm.io/gorm"` al import.

2. Extraer `cloneInternal` reusada por `Clone` (ya existente) y por `Update`:
```go
func cloneSessionInternal(sessionDao daos.SessionDaoInterface, sessionExerciseDao daos.SessionExerciseDaoInterface, ctx *gin.Context, original *dbs.Session, name, description *string) (*dbs.Session, error) {
	rows, err := sessionExerciseDao.FindBySession(ctx, original.ID)
	if err != nil {
		return nil, fmt.Errorf("error al leer ejercicios de la sesión original")
	}
	cloneName := original.Name + " (copia)"
	if name != nil {
		cloneName = *name
	}
	cloneDescription := original.Description
	if description != nil {
		cloneDescription = description
	}
	clone := &dbs.Session{OwnerID: original.OwnerID, Name: cloneName, Description: cloneDescription}
	if err := sessionDao.Create(ctx, clone); err != nil {
		return nil, fmt.Errorf("error al crear sesión clonada")
	}
	clonedRows := make([]dbs.SessionExercise, len(rows))
	for i, r := range rows {
		clonedRows[i] = dbs.SessionExercise{ExerciseID: r.ExerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	if err := sessionExerciseDao.ReplaceForSession(ctx, clone.ID, clonedRows); err != nil {
		return nil, fmt.Errorf("error al copiar ejercicios al clon")
	}
	return clone, nil
}
```

Reemplazar el cuerpo de `Clone` para usar el helper:
```go
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
	clone, err := cloneSessionInternal(s.sessionDao, s.sessionExerciseDao, ctx, existing, nil, nil)
	if err != nil {
		customlogger.Error(ctx, "error cloning session", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	return s.toResponse(ctx, clone)
}
```

3. Extender `Update` con la rama de divergencia (D8) — reemplazar el `Update` existente:
```go
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

	if req.ExcludeGroupIDs == nil || len(*req.ExcludeGroupIDs) == 0 {
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

	excludeGroupIDs := *req.ExcludeGroupIDs
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txSessionDao := daos.NewSessionDao(tx)
		txSessionExerciseDao := daos.NewSessionExerciseDao(tx)
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)

		clone, err := cloneSessionInternal(txSessionDao, txSessionExerciseDao, ctx, existing, req.CloneName, req.CloneDescription)
		if err != nil {
			return err
		}
		if err := txCalendarDao.RepointSessionForGroups(ctx, excludeGroupIDs, id, clone.ID); err != nil {
			return fmt.Errorf("error al repuntear grupos excluidos")
		}
		existing.Name = req.Name
		existing.Description = req.Description
		if err := txSessionDao.Update(ctx, existing); err != nil {
			return fmt.Errorf("error al editar sesión original")
		}
		return txSessionExerciseDao.ReplaceForSession(ctx, id, toSessionExerciseRows(req.Exercises))
	})
	if err != nil {
		customlogger.Error(ctx, "error in divergence-clone update", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión con exclusión de grupos")
	}
	return s.toResponse(ctx, existing)
}
```

- [ ] **Step 4: Corregir el único call site existente en `app.go`**

Ubicar `services.NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)` en `cmd/api/app/app.go` (agregado por el change anterior) y cambiarlo a `services.NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, groupCalendarDayDao, db)` — la variable `groupCalendarDayDao` todavía no existe en `app.go` en este punto del plan; agregar justo antes de esa línea: `groupCalendarDayDao := daos.NewGroupCalendarDayDao(db)` (Task 12 reusará esta misma variable para el resto del wiring de calendario, no la duplica).

- [ ] **Step 5: Correr todos los tests, verificar que pasan**

Run: `go build ./... && go vet ./... && go test ./cmd/api/services/... -run TestSessionService -v -count=1`
Expected: PASS (todos los tests existentes de `SessionService` siguen pasando con la firma nueva, más el/los tests de divergencia).

- [ ] **Step 6: Commit**

```bash
git add cmd/api/domains/session/session_request.go cmd/api/services/session_service.go cmd/api/services/session_service_test.go cmd/api/app/app.go
git commit -m "feat(calendar): add session divergence-clone on PUT with exclude_group_ids"
```

---

### Task 9: Limpiar source_plan_id al borrar un TrainingPlan

**Files:**
- Modify: `cmd/api/services/training_plan_service.go` (constructor +1 arg, `Delete` llama a `ClearSourcePlan` antes de borrar)
- Modify: `cmd/api/services/training_plan_service_test.go` (actualizar todas las llamadas a `NewTrainingPlanService`)
- Modify: `cmd/api/app/app.go` (única línea existente que llama a `NewTrainingPlanService`)

**Interfaces:**
- Consumes: `daos.GroupCalendarDaoInterface.ClearSourcePlan` (Task 3).
- Produces: `NewTrainingPlanService(trainingPlanDao, planDayDao, sessionDao, groupCalendarDayDao)` — firma nueva de 4 args (antes 3).

- [ ] **Step 1: Actualizar tests existentes**

`grep -n "NewTrainingPlanService(" cmd/api/services/training_plan_service_test.go` — cada ocurrencia con 3 args pasa a 4, agregando `&mockGroupCalendarDao{}` al final (mismo mock del paquete `services`, ya definido en `calendar_service_test.go`).

Agregar el test nuevo:
```go
func TestTrainingPlanService_Delete_ClearsSourcePlan(t *testing.T) {
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	cleared := false
	calDao := &mockGroupCalendarDao{clearSourcePlanFn: func(ctx *gin.Context, planID int64) error {
		cleared = true
		return nil
	}}
	svc := NewTrainingPlanService(planDao, &mockPlanDayDao{}, &mockSessionDao{}, calDao)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, cleared)
}
```

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/services/... -run TestTrainingPlanService_Delete -v -count=1`
Expected: FAIL (firma vieja, `cleared` nunca se pone en `true`).

- [ ] **Step 3: Implementar**

En `cmd/api/services/training_plan_service.go`:
```go
type trainingPlanService struct {
	trainingPlanDao     daos.TrainingPlanDaoInterface
	planDayDao          daos.PlanDayDaoInterface
	sessionDao          daos.SessionDaoInterface
	groupCalendarDayDao daos.GroupCalendarDaoInterface
}

func NewTrainingPlanService(
	trainingPlanDao daos.TrainingPlanDaoInterface,
	planDayDao daos.PlanDayDaoInterface,
	sessionDao daos.SessionDaoInterface,
	groupCalendarDayDao daos.GroupCalendarDaoInterface,
) TrainingPlanServiceInterface {
	return &trainingPlanService{
		trainingPlanDao: trainingPlanDao, planDayDao: planDayDao, sessionDao: sessionDao, groupCalendarDayDao: groupCalendarDayDao,
	}
}
```

Y en `Delete`, antes de `s.trainingPlanDao.Delete(ctx, id)`:
```go
	if err := s.groupCalendarDayDao.ClearSourcePlan(ctx, id); err != nil {
		customlogger.Error(ctx, "error clearing source_plan_id", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar plan")
	}
```

- [ ] **Step 4: Corregir el call site en `app.go`**

`services.NewTrainingPlanService(trainingPlanDao, planDayDao, sessionDao)` → agregar `, groupCalendarDayDao` (la variable ya existe en `app.go` desde Task 8, no duplicar `daos.NewGroupCalendarDayDao(db)`).

- [ ] **Step 5: Correr los tests, verificar que pasan**

Run: `go build ./... && go vet ./... && go test ./cmd/api/services/... -run TestTrainingPlanService -v -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/services/training_plan_service.go cmd/api/services/training_plan_service_test.go cmd/api/app/app.go
git commit -m "feat(calendar): clear source_plan_id when a training plan is deleted"
```

---

### Task 10: CalendarController

**Files:**
- Create: `cmd/api/controllers/calendar_controller.go`
- Test: `cmd/api/controllers/calendar_controller_test.go`

**Interfaces:**
- Consumes: `services.CalendarServiceInterface` (Tasks 4-7), `respondCatalogError` (change anterior, `catalog_common.go`).
- Produces: `CalendarController{GetRange,PutDay,DeleteDay,Stamp,Bulk,BulkClear,Shift,NextSession,CalendarSummary}` (9 handlers).

- [ ] **Step 1: Escribir el test**

`cmd/api/controllers/calendar_controller_test.go`:
```go
package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockCalendarService struct {
	getRangeFn        func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error)
	upsertDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error)
	deleteDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time) error
	stampFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error)
	bulkFn            func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error)
	bulkClearFn       func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error
	shiftFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error)
	nextSessionFn     func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error)
	calendarSummaryFn func(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error)
	assignedGroupsFn  func(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error)
}

func (m *mockCalendarService) GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
	return m.getRangeFn(ctx, groupID, callerID, from, to)
}
func (m *mockCalendarService) UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
	return m.upsertDayFn(ctx, groupID, callerID, date, req)
}
func (m *mockCalendarService) DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
	return m.deleteDayFn(ctx, groupID, callerID, date)
}
func (m *mockCalendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	return m.stampFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	return m.bulkFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	return m.bulkClearFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	return m.shiftFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	return m.nextSessionFn(ctx, userID)
}
func (m *mockCalendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	return m.calendarSummaryFn(ctx, userID)
}
func (m *mockCalendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	return m.assignedGroupsFn(ctx, sessionID)
}

func setupCalendarRouter(svc services.CalendarServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewCalendarController(svc)
	r.GET("/groups/:id/calendar", ctrl.GetRange)
	r.PUT("/groups/:id/calendar/:date", ctrl.PutDay)
	r.DELETE("/groups/:id/calendar/:date", ctrl.DeleteDay)
	r.POST("/groups/:id/calendar/stamp", ctrl.Stamp)
	r.POST("/groups/:id/calendar/bulk", ctrl.Bulk)
	r.POST("/groups/:id/calendar/bulk-clear", ctrl.BulkClear)
	r.POST("/groups/:id/calendar/shift", ctrl.Shift)
	r.GET("/users/:id/next-session", ctrl.NextSession)
	r.GET("/users/:id/calendar-summary", ctrl.CalendarSummary)
	return r
}

func TestCalendarController_GetRange_MissingDatesReturns400(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_GetRange_Success(t *testing.T) {
	svc := &mockCalendarService{getRangeFn: func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
		return []calendar.CalendarDayResponse{}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=2026-10-01&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_GetRange_Forbidden(t *testing.T) {
	svc := &mockCalendarService{getRangeFn: func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarForbidden
	}}
	router := setupCalendarRouter(svc, 99)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=2026-10-01&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCalendarController_PutDay_Success(t *testing.T) {
	svc := &mockCalendarService{upsertDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
		return &calendar.CalendarDayResponse{ID: 1, GroupID: groupID, Kind: req.Kind}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/2026-10-01", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_DeleteDay_Success(t *testing.T) {
	svc := &mockCalendarService{deleteDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time) error { return nil }}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/groups/1/calendar/2026-10-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCalendarController_Stamp_Conflict(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarStampConflict
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCalendarController_NextSession_NoContent(t *testing.T) {
	svc := &mockCalendarService{nextSessionFn: func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) { return nil, nil }}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/7/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCalendarController_NextSession_OtherUserForbidden(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/99/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
```

- [ ] **Step 2: Implementar el controller**

`cmd/api/controllers/calendar_controller.go`:
```go
package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type CalendarController interface {
	GetRange(c *gin.Context)
	PutDay(c *gin.Context)
	DeleteDay(c *gin.Context)
	Stamp(c *gin.Context)
	Bulk(c *gin.Context)
	BulkClear(c *gin.Context)
	Shift(c *gin.Context)
	NextSession(c *gin.Context)
	CalendarSummary(c *gin.Context)
}

type calendarController struct {
	calendarService services.CalendarServiceInterface
}

func NewCalendarController(calendarService services.CalendarServiceInterface) CalendarController {
	return &calendarController{calendarService: calendarService}
}

func mapCalendarError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrCalendarGroupNotFound):
		return http.StatusNotFound, "grupo no encontrado"
	case errors.Is(err, services.ErrCalendarPlanNotFound):
		return http.StatusNotFound, "plan no encontrado"
	case errors.Is(err, services.ErrCalendarForbidden):
		return http.StatusForbidden, "no autorizado"
	case errors.Is(err, services.ErrCalendarPlanForbidden):
		return http.StatusForbidden, "el plan no pertenece al entrenador dueño del grupo"
	case errors.Is(err, services.ErrCalendarInvalidKind):
		return http.StatusUnprocessableEntity, "kind inválido"
	case errors.Is(err, services.ErrCalendarFieldMismatch):
		return http.StatusUnprocessableEntity, "combinación de campos inválida"
	case errors.Is(err, services.ErrCalendarInvalidCancelTransition):
		return http.StatusUnprocessableEntity, "solo se puede cancelar un día en training"
	case errors.Is(err, services.ErrCalendarStampConflict):
		return http.StatusConflict, "hay fechas con contenido existente"
	case errors.Is(err, services.ErrCalendarShiftCollision):
		return http.StatusConflict, "el corrimiento haría chocar dos fechas"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondCalendarError(c *gin.Context, err error) {
	status, message := mapCalendarError(err)
	respondCatalogError(c, status, message)
}

func (cc *calendarController) GetRange(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	fromStr, toStr := c.Query("from"), c.Query("to")
	if fromStr == "" || toStr == "" {
		respondCatalogError(c, http.StatusBadRequest, "from y to son obligatorios")
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "from debe tener formato YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "to debe tener formato YYYY-MM-DD")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.GetRange(c, groupID, callerID, from, to)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (cc *calendarController) PutDay(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	date, err := time.Parse("2006-01-02", c.Param("date"))
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "date debe tener formato YYYY-MM-DD")
		return
	}
	var req calendar.CalendarDayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.UpsertDay(c, groupID, callerID, date, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (cc *calendarController) DeleteDay(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	date, err := time.Parse("2006-01-02", c.Param("date"))
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "date debe tener formato YYYY-MM-DD")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := cc.calendarService.DeleteDay(c, groupID, callerID, date); err != nil {
		respondCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (cc *calendarController) Stamp(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.StampRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Stamp(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (cc *calendarController) Bulk(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Bulk(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (cc *calendarController) BulkClear(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.BulkClearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := cc.calendarService.BulkClear(c, groupID, callerID, req); err != nil {
		respondCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (cc *calendarController) Shift(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.ShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Shift(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (cc *calendarController) NextSession(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if userID != callerID {
		respondCatalogError(c, http.StatusForbidden, "no podés consultar los datos de otro usuario")
		return
	}
	resp, err := cc.calendarService.NextSession(c, userID)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	if resp == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (cc *calendarController) CalendarSummary(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if userID != callerID {
		respondCatalogError(c, http.StatusForbidden, "no podés consultar los datos de otro usuario")
		return
	}
	resp, err := cc.calendarService.CalendarSummary(c, userID)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 3: Correr el test, verificar que pasa**

Run: `go test ./cmd/api/controllers/... -run TestCalendarController -v -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/controllers/calendar_controller.go cmd/api/controllers/calendar_controller_test.go
git commit -m "feat(calendar): add CalendarController"
```

---

### Task 11: Extender SessionController (AssignedGroups + PUT con divergencia)

**Files:**
- Modify: `cmd/api/controllers/session_controller.go` (constructor +1 arg, +1 handler)
- Modify: `cmd/api/controllers/session_controller_test.go`
- Modify: `cmd/api/app/app.go` (único call site de `NewSessionController`)

**Interfaces:**
- Consumes: `services.CalendarServiceInterface.AssignedGroups` (Task 7).
- Produces: `NewSessionController(sessionService, calendarService)` — firma nueva de 2 args (antes 1); `SessionController` gana el método `AssignedGroups(c *gin.Context)`.

- [ ] **Step 1: Actualizar tests existentes + agregar el nuevo**

`grep -n "NewSessionController(" cmd/api/controllers/session_controller_test.go` — la única ocurrencia pasa de `NewSessionController(svc)` a `NewSessionController(svc, &mockCalendarService{})` (mock ya definido en `calendar_controller_test.go`, mismo paquete `controllers`).

Agregar:
```go
func TestSessionController_AssignedGroups_Success(t *testing.T) {
	sessionSvc := &mockSessionService{}
	calendarSvc := &mockCalendarService{assignedGroupsFn: func(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
		return []calendar.CalendarSummaryItem{{GroupID: 1, GroupName: "Grupo 1"}}, nil
	}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctrl := NewSessionController(sessionSvc, calendarSvc)
	r.GET("/sessions/:id/assigned-groups", ctrl.AssignedGroups)
	req := httptest.NewRequest(http.MethodGet, "/sessions/1/assigned-groups", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
```

(Agregar `"simple-arq-golang/cmd/api/domains/calendar"` al import de `session_controller_test.go` si no está.)

- [ ] **Step 2: Correr el test, verificar que falla**

Run: `go test ./cmd/api/controllers/... -run TestSessionController -v -count=1`
Expected: FAIL (firma vieja).

- [ ] **Step 3: Implementar**

En `cmd/api/controllers/session_controller.go`:
```go
type sessionController struct {
	sessionService  services.SessionServiceInterface
	calendarService services.CalendarServiceInterface
}

func NewSessionController(sessionService services.SessionServiceInterface, calendarService services.CalendarServiceInterface) SessionController {
	return &sessionController{sessionService: sessionService, calendarService: calendarService}
}
```

Agregar el método (y agregarlo a la interfaz `SessionController`):
```go
	AssignedGroups(c *gin.Context)
```
```go
func (sc *sessionController) AssignedGroups(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := sc.calendarService.AssignedGroups(c, id)
	if err != nil {
		respondCatalogError(c, http.StatusInternalServerError, "error interno")
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

El `Update` existente de `SessionController` NO necesita cambios de código — `session.SessionRequest` ya trae los 3 campos nuevos desde Task 8, `c.ShouldBindJSON` los captura automáticamente, y `sessionService.Update` ya sabe interpretarlos.

- [ ] **Step 4: Corregir el call site en `app.go`**

`controllers.NewSessionController(sessionService)` → `controllers.NewSessionController(sessionService, calendarService)` — la variable `calendarService` todavía no existe en `app.go` en este punto; se crea recién en Task 12. **Para que esta task sea buildable por sí sola, adelantar en `app.go` la construcción mínima de `calendarService`** (Task 12 la reordena/reusa, no la duplica):
```go
calendarService := services.NewCalendarService(groupCalendarDayDao, groupDao, teamDao, groupUserDao, teamUserDao, trainingPlanDao, planDayDao, sessionDao, db)
```
ubicada antes de la construcción de `sessionController`, usando variables (`groupDao`, `teamDao`, `groupUserDao`, `teamUserDao`) que ya existen en `app.go` desde el dominio de equipos (buscar con `grep -n "groupDao :=\|teamDao :=\|groupUserDao :=\|teamUserDao :="  cmd/api/app/app.go` para confirmar los nombres exactos de esas variables ya wireadas).

- [ ] **Step 5: Correr los tests, verificar que pasan**

Run: `go build ./... && go vet ./... && go test ./cmd/api/controllers/... -run TestSessionController -v -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/controllers/session_controller.go cmd/api/controllers/session_controller_test.go cmd/api/app/app.go
git commit -m "feat(calendar): add SessionController.AssignedGroups"
```

---

### Task 12: Wiring final (rutas de calendario)

**Files:**
- Modify: `cmd/api/app/app.go` (agregar campo `calendarController` a `Application`, mover/confirmar la construcción de `calendarService`/`calendarController` en un solo lugar coherente — sin duplicar lo que Task 11 ya adelantó)
- Modify: `cmd/api/app/url_mappings.go` (10 rutas nuevas: 9 de calendario + `assigned-groups`)

**Interfaces:**
- Consumes: todo lo de Tasks 1-11.

- [ ] **Step 1: Revisar y consolidar el wiring de `app.go`**

Confirmar que `app.go` (después de Tasks 8, 9 y 11) ya tiene: `groupCalendarDayDao`, `calendarService`, `sessionController` (con 2 args) y `trainingPlanService`/`trainingPlanController` (con el 4to arg). Agregar:
- Campo `calendarController controllers.CalendarController` al struct `Application`.
- `calendarController := controllers.NewCalendarController(calendarService)` (reusa la instancia de `calendarService` ya creada en Task 11 — **no crear una segunda**).
- `calendarController: calendarController,` en el `return &Application{...}`.

- [ ] **Step 2: Agregar las 10 rutas en `url_mappings.go`**

Después del bloque de rutas del catálogo (`/api/v1/training-plans/:id/clone`), agregar, detrás de `AuthMiddleware()`:
```go
// Group calendar
r.GET("/api/v1/groups/:id/calendar", app.calendarController.GetRange)
r.PUT("/api/v1/groups/:id/calendar/:date", app.calendarController.PutDay)
r.DELETE("/api/v1/groups/:id/calendar/:date", app.calendarController.DeleteDay)
r.POST("/api/v1/groups/:id/calendar/stamp", app.calendarController.Stamp)
r.POST("/api/v1/groups/:id/calendar/bulk", app.calendarController.Bulk)
r.POST("/api/v1/groups/:id/calendar/bulk-clear", app.calendarController.BulkClear)
r.POST("/api/v1/groups/:id/calendar/shift", app.calendarController.Shift)

// Runner calendar views
r.GET("/api/v1/users/:id/next-session", app.calendarController.NextSession)
r.GET("/api/v1/users/:id/calendar-summary", app.calendarController.CalendarSummary)

// Session divergence-clone helper
r.GET("/api/v1/sessions/:id/assigned-groups", app.sessionController.AssignedGroups)
```

- [ ] **Step 3: Verificar**

Run: `go build ./... && go vet ./... && go test ./cmd/api/app/... -run TestPingRouteExists -v -count=1`
Expected: verde.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/app/app.go cmd/api/app/url_mappings.go
git commit -m "feat(calendar): wire CalendarController and its 10 routes"
```

---

### Task 13: Swagger y documentación

**Files:**
- Modify: `cmd/api/controllers/calendar_controller.go` (godoc en los 9 handlers)
- Modify: `cmd/api/controllers/session_controller.go` (godoc en `AssignedGroups`, actualizar el de `Update` si menciona el body)
- Modify: `cmd/api/docs/{docs.go,swagger.json,swagger.yaml}` (regenerados)
- Modify: `README.md`

- [ ] **Step 1: Agregar anotaciones godoc**

Mismo criterio que la Task 13 del change anterior (`catalogo-planes-entrenamiento`) — `@Failure` sin `{object}` (shape `{"message":...}`), tags `calendar` para los handlers de `CalendarController`. Ejemplo para `GetRange`:
```go
// GetRange godoc
// @Summary      Calendario de un grupo en un rango de fechas
// @Tags         calendar
// @Produce      json
// @Param        id    path   int     true   "Group ID"
// @Param        from  query  string  true   "Fecha desde (YYYY-MM-DD)"
// @Param        to    query  string  true   "Fecha hasta (YYYY-MM-DD)"
// @Success      200  {array}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Router       /api/v1/groups/{id}/calendar [get]
func (cc *calendarController) GetRange(c *gin.Context) {
```
Repetir el mismo criterio para los 8 handlers restantes de `CalendarController` y para `SessionController.AssignedGroups`, usando las rutas exactas de `url_mappings.go` (Task 12).

- [ ] **Step 2: Regenerar swagger**

Run: `/home/adiazcriv/go/bin/swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`

- [ ] **Step 3: Actualizar README**

Agregar las 10 rutas nuevas a la tabla de endpoints, mismo formato que las del change anterior.

- [ ] **Step 4: Verificar y commitear**

Run: `go build ./...`
```bash
git add cmd/api/controllers/calendar_controller.go cmd/api/controllers/session_controller.go cmd/api/docs README.md
git commit -m "docs(calendar): add swagger annotations and README endpoint table"
```

---

### Task 14: Verificación final

**Files:** ninguno nuevo por defecto — solo verificación, salvo que aparezca un gap de cobertura real (mismo criterio que el change anterior).

- [ ] **Step 1: Suite completa sin DB**

Run: `go build ./... && go vet ./... && go test ./... -count=1`
Expected: verde.

- [ ] **Step 2: Suite completa con Postgres real**

```bash
export TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test
go test ./... -count=1
```
Expected: verde, incluidos los DAO tests de `GroupCalendarDayDao` (Task 3).

- [ ] **Step 3: Coverage**

Run: `make coverage-with-db`. Si el total del proyecto queda por debajo de 80%, agregar tests a los archivos de este change (`calendar_service_test.go`, `calendar_controller_test.go`, `group_calendar_day_dao_test.go`) cubriendo las ramas de error/edge case que falten — mismo criterio que la Task 14 del change anterior (gap real, se cierra ahí mismo, no se defiere).

- [ ] **Step 4: Chequeo estático de rutas**

Confirmar que las 10 rutas nuevas de `url_mappings.go` aparecen en `cmd/api/docs/swagger.json` con el método correcto — sustituto del end-to-end manual si no hay servidor disponible (mismo criterio que el change anterior).

- [ ] **Step 5: Commit final si hubo ajustes**

```bash
git add -A
git commit -m "test(calendar): close coverage gaps found in final verification"
```
(Solo si hubo cambios reales — no crear un commit vacío.)
