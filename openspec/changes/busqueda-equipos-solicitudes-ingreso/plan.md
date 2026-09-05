# Búsqueda de Equipos y Solicitudes de Ingreso Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a corredor search for public teams and request to join one without an entrenador inviting them first, reusing the exact same payment-gate and default-group-assignment logic that already governs `AddUser`/`AcceptInvitation`.

**Architecture:** New `join_requests` table + `JoinRequestServiceInterface`/`JoinRequestDaoInterface`/`join_request_controller` following this repo's `Controllers → Services → DAOs` layering (no delegate — nothing here composes two services, same shape as `invitation_service.go`). `teams` gains `Visible`/`IsPublic` columns and a `Search` method on the existing `TeamServiceInterface`/`TeamDaoInterface`/`team_controller`. A new package-level `AssignToDefaultGroup` function is extracted from `invitation_service.go` so both `AcceptInvitation` and the new `join_request_service.Accept` share it.

**Tech Stack:** Go 1.26, Gin, GORM/PostgreSQL, testify (assert/require), manual function-field mocks (no mocking library) — all pre-existing in this repo, no new dependencies.

**Spec:** `openspec/changes/busqueda-equipos-solicitudes-ingreso/{proposal,design}.md` and `specs/{team-search,team-join-requests}/spec.md`

## Global Constraints

- Branch: work happens directly on `feature/busqueda-equipos-solicitudes-ingreso` (already created, spec already committed there).
- Layering: `.agentics/CONVENTIONS.md` — controllers never call DAOs directly, no service-to-service imports. Every new DAO method takes `ctx *gin.Context` as first param (existing convention, even though most implementations ignore it).
- Error codes: SCREAMING_SNAKE via Go sentinel errors + `errors.Is` in controllers (the newer convention from the photos feature), never the older exact-string-match convention.
- `teams.visible`/`teams.is_public`: both `not null default true`.
- `join_requests.status` reuses `constants.InvitationStatus` values (`pending`/`accepted`/`rejected`) — no new enum, no `cancelled` value (cancel hard-deletes the row).
- Search pagination: `page` 1-indexed, fixed page size `20`, `has_more` derived from fetching `pageSize+1` rows — no `total`. Only `GET /teams/search` paginates; join-request listing endpoints do not.
- `Accept` mirrors `invitation_service.AcceptInvitation`'s exact sequential structure (existingMember guard → gate → best-effort group assignment → independent status update) — no new transaction-wrapping pattern.
- Every new/changed exported Go identifier must compile against the whole repo (`go build ./...`) and `go vet ./...` before each commit; `go test ./...` must stay green throughout (daos tests skip without `TEST_DB_HOST`, that's expected locally).
- Commit messages in English (Conventional Commits, per `CLAUDE.md`), one per task.

---

### Task 1: DB models — `Team.Visible`/`IsPublic` + `dbs.JoinRequest`

**Files:**
- Modify: `cmd/api/domains/dbs/team.go`
- Create: `cmd/api/domains/dbs/join_request.go`
- Modify: `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate list, near `&dbs.Team{}` / `&dbs.Invitation{}` around line 79/83)

**Interfaces:**
- Produces: `dbs.Team.Visible bool`, `dbs.Team.IsPublic bool`; `dbs.JoinRequest{ID, TeamID, RunnerID, Status, CreatedAt, UpdatedAt}`, table `join_requests`.

- [ ] **Step 1: Add the two new fields to `dbs.Team`**

In `cmd/api/domains/dbs/team.go`, add after `ShowGroupsToRunners`:

```go
	ShowGroupsToRunners bool       `gorm:"column:show_groups_to_runners;not null;default:false"` // Si los corredores ven a qué grupo pertenece cada compañero
	Visible             bool       `gorm:"column:visible;not null;default:true"`                 // Si aparece en resultados de búsqueda de equipos
	IsPublic            bool       `gorm:"column:is_public;not null;default:true"`               // Si acepta solicitudes de ingreso ("Solicitar unirse")
```

- [ ] **Step 2: Create `dbs.JoinRequest`**

```go
package dbs

import "time"

// JoinRequest representa la solicitud de un corredor para unirse a un equipo
// público. Status reusa los mismos 3 valores que constants.InvitationStatus
// (pending/accepted/rejected) — cancelar una solicitud propia borra la fila
// en vez de agregar un 4to estado.
type JoinRequest struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TeamID    int64     `gorm:"column:team_id;not null"`
	RunnerID  int64     `gorm:"column:runner_id;not null"`
	Status    string    `gorm:"column:status;not null;default:pending"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (JoinRequest) TableName() string {
	return "join_requests"
}
```

- [ ] **Step 3: Register `dbs.JoinRequest{}` in AutoMigrate**

In `cmd/api/infrastructure/postgresdb/postgres.go`, add `&dbs.JoinRequest{},` to the `db.AutoMigrate(...)` call, next to `&dbs.Team{}`/`&dbs.Invitation{}`.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/domains/dbs/team.go cmd/api/domains/dbs/join_request.go cmd/api/infrastructure/postgresdb/postgres.go
git commit -m "feat(teams): add visible/is_public columns and join_requests table"
```

---

### Task 2: Extract `AssignToDefaultGroup`, update `AcceptInvitation`

**Files:**
- Create: `cmd/api/services/team_group_assignment.go`
- Create: `cmd/api/services/team_group_assignment_test.go`
- Modify: `cmd/api/services/invitation_service.go` (replace `assignInviteeToGroup` method + its call site)

**Interfaces:**
- Consumes: `daos.GroupDaoInterface.GetByTeamID`, `daos.GroupUserDaoInterface.FindByGroupAndUser`/`Create` (both already exist).
- Produces: `AssignToDefaultGroup(ctx *gin.Context, groupDao daos.GroupDaoInterface, groupUserDao daos.GroupUserDaoInterface, teamID int64, groupID *int64, userID int64)` — used by Task 7 (`join_request_service.Accept`).

- [ ] **Step 1: Write the failing tests**

```go
// cmd/api/services/team_group_assignment_test.go
package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
)

func TestAssignToDefaultGroup_ExplicitGroupID(t *testing.T) {
	created := false
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			created = true
			assert.Equal(t, int64(42), gu.GroupID)
			return nil
		},
	}

	groupID := int64(42)
	AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)

	assert.True(t, created)
}

func TestAssignToDefaultGroup_FallsBackToMainGroup(t *testing.T) {
	groupDao := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 1, IsMain: false}, {ID: 2, IsMain: true}}, nil
		},
	}
	var createdGroupID int64
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			createdGroupID = gu.GroupID
			return nil
		},
	}

	AssignToDefaultGroup(nil, groupDao, groupUserDao, 1, nil, 7)

	assert.Equal(t, int64(2), createdGroupID)
}

func TestAssignToDefaultGroup_NoDefaultGroup_LogsAndReturns(t *testing.T) {
	groupDao := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 1, IsMain: false}}, nil
		},
	}
	called := false
	groupUserDao := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			called = true
			return nil
		},
	}

	AssignToDefaultGroup(nil, groupDao, groupUserDao, 1, nil, 7)

	assert.False(t, called)
}

func TestAssignToDefaultGroup_AlreadyMember_DoesNotDuplicate(t *testing.T) {
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{GroupID: groupID, UserID: userID}, nil
		},
	}
	called := false
	groupUserDao.createFn = func(ctx *gin.Context, gu *dbs.GroupUser) error {
		called = true
		return nil
	}

	groupID := int64(5)
	AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)

	assert.False(t, called)
}

func TestAssignToDefaultGroup_CreateFails_LogsAndReturns(t *testing.T) {
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			return errors.New("db error")
		},
	}

	groupID := int64(5)
	assert.NotPanics(t, func() {
		AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/services/... -run TestAssignToDefaultGroup -v`
Expected: FAIL — `AssignToDefaultGroup` undefined.

- [ ] **Step 3: Create `team_group_assignment.go`**

```go
package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// AssignToDefaultGroup da de alta a userID en groupID, o en el grupo principal
// (IsMain) de teamID si groupID es nil. Best-effort: nunca devuelve error ni
// bloquea al caller, solo loguea si falla — extraído de invitation_service.go
// (antes assignInviteeToGroup, atado a *dbs.Invitation) para que join_request_service
// use la misma lógica sin duplicarla.
func AssignToDefaultGroup(
	ctx *gin.Context,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	teamID int64,
	groupID *int64,
	userID int64,
) {
	targetGroupID := groupID

	if targetGroupID == nil {
		groups, err := groupDao.GetByTeamID(ctx, teamID)
		if err != nil {
			customlogger.Error(ctx, "error finding team groups for default group assignment", err,
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
				customlogger.TagMethod("AssignToDefaultGroup"))
			return
		}
		for _, g := range groups {
			if g.IsMain {
				id := g.ID
				targetGroupID = &id
				break
			}
		}
		if targetGroupID == nil {
			customlogger.Warn(ctx, "no default group found for team on membership assignment",
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
				customlogger.TagMethod("AssignToDefaultGroup"))
			return
		}
	}

	existingGroupMember, err := groupUserDao.FindByGroupAndUser(ctx, *targetGroupID, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking group membership on membership assignment", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AssignToDefaultGroup"))
		return
	}
	if existingGroupMember != nil {
		return
	}

	groupUser := &dbs.GroupUser{
		GroupID:   *targetGroupID,
		UserID:    userID,
		DateStart: time.Now(),
	}
	if err := groupUserDao.Create(ctx, groupUser); err != nil {
		customlogger.Error(ctx, "error creating group_user on membership assignment", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("group_id", fmt.Sprintf("%d", *targetGroupID)),
			customlogger.TagMethod("AssignToDefaultGroup"))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/services/... -run TestAssignToDefaultGroup -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Update `invitation_service.go` to use the extracted function**

In `cmd/api/services/invitation_service.go`:
1. Delete the entire `assignInviteeToGroup` method (currently lines 471-523, the block starting with `// assignInviteeToGroup da de alta...` through its closing `}`).
2. Change the call site (currently `s.assignInviteeToGroup(ctx, inv, userID)`, around line 411) to:

```go
	AssignToDefaultGroup(ctx, s.groupDao, s.groupUserDao, inv.TeamID, inv.GroupID, userID)
```

- [ ] **Step 6: Run the full invitation test suite to confirm no behavior change**

Run: `go test ./cmd/api/services/... -run TestInvitationService -v`
Expected: PASS, same as before the refactor.

- [ ] **Step 7: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all green.

- [ ] **Step 8: Commit**

```bash
git add cmd/api/services/team_group_assignment.go cmd/api/services/team_group_assignment_test.go cmd/api/services/invitation_service.go
git commit -m "refactor(services): extract AssignToDefaultGroup from invitation_service"
```

---

### Task 3: `JoinRequestDaoInterface` + implementation

**Files:**
- Create: `cmd/api/daos/join_request_dao.go`
- Create: `cmd/api/daos/join_request_dao_test.go`

**Interfaces:**
- Consumes: `dbs.JoinRequest` (Task 1), `constants.InvitationStatus` (existing).
- Produces: `JoinRequestDaoInterface{Create, FindByID, FindPendingByTeamAndUser, FindPendingByTeam, FindByUser, UpdateStatus, Delete, CountPendingByOwner}` — used by Task 6/7/8 (`join_request_service`).

- [ ] **Step 1: Write the failing tests**

```go
// cmd/api/daos/join_request_dao_test.go
package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestJoinRequestDao_ImplementsInterface(t *testing.T) {
	dao := NewJoinRequestDao(&gorm.DB{})
	var iface JoinRequestDaoInterface = dao
	_ = iface
}

func testJoinRequestTeamAndOwner(db *gorm.DB, suffix string) (*dbs.Team, *dbs.User) {
	owner := persistUser(db, "jr-owner-"+suffix+"@test.com", "3000000"+suffix)
	team := testTeam(db, "equipo_jr_"+suffix, owner.ID)
	return team, owner
}

func TestJoinRequestDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "1")
	runner := persistUser(db, "jr-runner-1@test.com", "40000001")

	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	err := dao.Create(nil, jr)

	require.NoError(t, err)
	assert.NotZero(t, jr.ID)
}

func TestJoinRequestDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "2")
	runner := persistUser(db, "jr-runner-2@test.com", "40000002")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	found, err := dao.FindByID(nil, jr.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, runner.ID, found.RunnerID)
}

func TestJoinRequestDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestJoinRequestDao_FindPendingByTeamAndUser_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "3")
	runner := persistUser(db, "jr-runner-3@test.com", "40000003")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}))

	found, err := dao.FindPendingByTeamAndUser(nil, team.ID, runner.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
}

func TestJoinRequestDao_FindPendingByTeamAndUser_IgnoresResolved(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "4")
	runner := persistUser(db, "jr-runner-4@test.com", "40000004")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))
	require.NoError(t, dao.UpdateStatus(nil, jr.ID, string(constants.InvitationStatusRejected)))

	found, err := dao.FindPendingByTeamAndUser(nil, team.ID, runner.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestJoinRequestDao_FindPendingByTeam_OnlyPending(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "5")
	runnerA := persistUser(db, "jr-runner-5a@test.com", "40000005")
	runnerB := persistUser(db, "jr-runner-5b@test.com", "40000006")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: team.ID, RunnerID: runnerA.ID, Status: string(constants.InvitationStatusPending)}))
	resolved := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, resolved))
	require.NoError(t, dao.UpdateStatus(nil, resolved.ID, string(constants.InvitationStatusAccepted)))

	found, err := dao.FindPendingByTeam(nil, team.ID)

	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, runnerA.ID, found[0].RunnerID)
}

func TestJoinRequestDao_FindByUser_AllStatuses(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	teamA, _ := testJoinRequestTeamAndOwner(db, "6")
	teamB, _ := testJoinRequestTeamAndOwner(db, "7")
	runner := persistUser(db, "jr-runner-6@test.com", "40000007")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}))
	rejected := &dbs.JoinRequest{TeamID: teamB.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, rejected))
	require.NoError(t, dao.UpdateStatus(nil, rejected.ID, string(constants.InvitationStatusRejected)))

	found, err := dao.FindByUser(nil, runner.ID)

	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestJoinRequestDao_UpdateStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "8")
	runner := persistUser(db, "jr-runner-8@test.com", "40000008")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	err := dao.UpdateStatus(nil, jr.ID, string(constants.InvitationStatusAccepted))

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, jr.ID)
	assert.Equal(t, string(constants.InvitationStatusAccepted), found.Status)
}

func TestJoinRequestDao_Delete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "9")
	runner := persistUser(db, "jr-runner-9@test.com", "40000009")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	err := dao.Delete(nil, jr.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, jr.ID)
	assert.Nil(t, found)
}

func TestJoinRequestDao_CountPendingByOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	owner := persistUser(db, "jr-owner-count@test.com", "40000010")
	teamA := testTeam(db, "equipo_jr_count_a", owner.ID)
	teamB := testTeam(db, "equipo_jr_count_b", owner.ID)
	runnerA := persistUser(db, "jr-runner-count-a@test.com", "40000011")
	runnerB := persistUser(db, "jr-runner-count-b@test.com", "40000012")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runnerA.ID, Status: string(constants.InvitationStatusPending)}))
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamB.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}))
	resolved := &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, resolved))
	require.NoError(t, dao.UpdateStatus(nil, resolved.ID, string(constants.InvitationStatusRejected)))

	count, err := dao.CountPendingByOwner(nil, owner.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
```

- [ ] **Step 2: Run tests to verify they fail (or skip without TEST_DB_HOST)**

Run: `go test ./cmd/api/daos/... -run TestJoinRequestDao -v`
Expected: FAIL — `NewJoinRequestDao`/`JoinRequestDaoInterface` undefined (or SKIP if `TEST_DB_HOST` isn't set locally — either is fine at this step, the point is confirming it doesn't compile yet).

- [ ] **Step 3: Implement `join_request_dao.go`**

```go
package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// JoinRequestDaoInterface define las operaciones de acceso a datos para
// solicitudes de ingreso de un corredor a un equipo.
type JoinRequestDaoInterface interface {
	Create(ctx *gin.Context, jr *dbs.JoinRequest) error
	FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error)
	FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error)
	FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error)
	FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error)
	UpdateStatus(ctx *gin.Context, id int64, status string) error
	Delete(ctx *gin.Context, id int64) error
	CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error)
}

type joinRequestDao struct {
	DB *gorm.DB
}

// NewJoinRequestDao crea una nueva instancia de JoinRequestDao.
func NewJoinRequestDao(database *gorm.DB) JoinRequestDaoInterface {
	return &joinRequestDao{DB: database}
}

func (d *joinRequestDao) Create(ctx *gin.Context, jr *dbs.JoinRequest) error {
	return d.DB.Create(jr).Error
}

func (d *joinRequestDao) FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
	var jr dbs.JoinRequest
	err := d.DB.Where("id = ?", id).First(&jr).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding join request: %w", err)
	}
	return &jr, nil
}

func (d *joinRequestDao) FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
	var jr dbs.JoinRequest
	err := d.DB.Where("team_id = ? AND runner_id = ? AND status = ?", teamID, runnerID, string(constants.InvitationStatusPending)).First(&jr).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding pending join request: %w", err)
	}
	return &jr, nil
}

func (d *joinRequestDao) FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
	var requests []dbs.JoinRequest
	err := d.DB.Where("team_id = ? AND status = ?", teamID, string(constants.InvitationStatusPending)).Order("id").Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("error finding pending join requests: %w", err)
	}
	return requests, nil
}

func (d *joinRequestDao) FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
	var requests []dbs.JoinRequest
	err := d.DB.Where("runner_id = ?", runnerID).Order("id DESC").Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("error finding join requests by user: %w", err)
	}
	return requests, nil
}

func (d *joinRequestDao) UpdateStatus(ctx *gin.Context, id int64, status string) error {
	return d.DB.Model(&dbs.JoinRequest{}).Where("id = ?", id).Update("status", status).Error
}

func (d *joinRequestDao) Delete(ctx *gin.Context, id int64) error {
	return d.DB.Delete(&dbs.JoinRequest{}, id).Error
}

func (d *joinRequestDao) CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error) {
	var count int64
	err := d.DB.Model(&dbs.JoinRequest{}).
		Joins("JOIN teams ON teams.id = join_requests.team_id").
		Where("teams.owner_id = ? AND join_requests.status = ?", ownerID, string(constants.InvitationStatusPending)).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting pending join requests: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TEST_DB_HOST=localhost go test ./cmd/api/daos/... -run TestJoinRequestDao -v` (or however this repo's `make test-db-up` exposes it locally — see `docs/TESTING.md`; skips cleanly without it).
Expected: PASS (all scenarios), or SKIP if no local Postgres — either is acceptable locally, CI has Postgres.

- [ ] **Step 5: Full build/vet**

Run: `go build ./... && go vet ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/daos/join_request_dao.go cmd/api/daos/join_request_dao_test.go
git commit -m "feat(daos): add JoinRequestDao"
```

---

### Task 4: `TeamDaoInterface.SearchPublic`

**Files:**
- Modify: `cmd/api/daos/team_dao.go`
- Modify: `cmd/api/daos/team_dao_test.go`

**Interfaces:**
- Produces: `daos.TeamSearchFilters{Name, Level, Country, Province, City string}`; `TeamDaoInterface.SearchPublic(ctx *gin.Context, filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error)` — used by Task 9 (`team_service.Search`).

- [ ] **Step 1: Write the failing tests**

```go
// append to cmd/api/daos/team_dao_test.go

func TestTeamDao_SearchPublic_ExcludesInvisible(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-1@test.com", "50000001")
	caller := persistUser(db, "search-caller-1@test.com", "50000002")
	visible := testTeam(db, "equipo_visible_1", owner.ID)
	require.NoError(t, dao.Update(nil, visible))
	invisible := testTeam(db, "equipo_invisible_1", owner.ID)
	invisible.Visible = false
	require.NoError(t, dao.Update(nil, invisible))

	results, hasMore, err := dao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 20)

	require.NoError(t, err)
	assert.False(t, hasMore)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "equipo_visible_1")
	assert.NotContains(t, names, "equipo_invisible_1")
}

func TestTeamDao_SearchPublic_ExcludesCallerMembership(t *testing.T) {
	db := testutils.SetupTestDB(t)
	teamDao := NewTeamDao(db)
	teamUserDao := NewTeamUserDao(db)
	owner := persistUser(db, "search-owner-2@test.com", "50000003")
	caller := persistUser(db, "search-caller-2@test.com", "50000004")
	team := testTeam(db, "equipo_member_test", owner.ID)
	require.NoError(t, teamUserDao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: caller.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}))

	results, _, err := teamDao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 20)

	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, team.ID, r.ID)
	}
}

func TestTeamDao_SearchPublic_FiltersByName(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-3@test.com", "50000005")
	caller := persistUser(db, "search-caller-3@test.com", "50000006")
	testTeam(db, "runners_unicos_del_sur", owner.ID)
	testTeam(db, "otro_equipo_cualquiera", owner.ID)

	results, _, err := dao.SearchPublic(nil, TeamSearchFilters{Name: "unicos"}, caller.ID, 1, 20)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "runners_unicos_del_sur", results[0].Name)
}

func TestTeamDao_SearchPublic_HasMore(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-4@test.com", "50000007")
	caller := persistUser(db, "search-caller-4@test.com", "50000008")
	for i := 0; i < 3; i++ {
		testTeam(db, fmt.Sprintf("equipo_paginado_%d", i), owner.ID)
	}

	results, hasMore, err := dao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 2)

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, hasMore)
}
```

Add `"fmt"` and `"time"` to this test file's imports if not already present (`time` is already imported per the existing file header).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/daos/... -run TestTeamDao_SearchPublic -v`
Expected: FAIL — `TeamSearchFilters`/`SearchPublic` undefined.

- [ ] **Step 3: Implement `SearchPublic`**

In `cmd/api/daos/team_dao.go`, add to the interface and implementation:

```go
// TeamSearchFilters agrupa los filtros opcionales de búsqueda de equipos.
type TeamSearchFilters struct {
	Name     string
	Level    string
	Country  string
	Province string
	City     string
}
```

Add to `TeamDaoInterface`:

```go
	SearchPublic(ctx *gin.Context, filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error)
```

Add the implementation:

```go
// SearchPublic busca equipos visible=true, excluyendo aquellos donde callerID
// ya es miembro activo. Pide pageSize+1 filas para derivar hasMore sin un
// COUNT(*) adicional.
func (d *teamDao) SearchPublic(ctx *gin.Context, filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error) {
	query := d.DB.Model(&dbs.Team{}).
		Where("teams.visible = true AND teams.deleted_at IS NULL").
		Where("teams.id NOT IN (?)", d.DB.Model(&dbs.TeamUser{}).Select("team_id").Where("user_id = ? AND deleted_at IS NULL", callerID))

	if filters.Name != "" {
		query = query.Where("teams.name ILIKE ?", "%"+filters.Name+"%")
	}
	if filters.Level != "" {
		query = query.Where("teams.level = ?", filters.Level)
	}
	if filters.Country != "" {
		query = query.Where("teams.country = ?", filters.Country)
	}
	if filters.Province != "" {
		query = query.Where("teams.province = ?", filters.Province)
	}
	if filters.City != "" {
		query = query.Where("teams.city = ?", filters.City)
	}

	var teams []dbs.Team
	offset := (page - 1) * pageSize
	err := query.Order("teams.id").Offset(offset).Limit(pageSize + 1).Find(&teams).Error
	if err != nil {
		return nil, false, fmt.Errorf("error searching teams: %w", err)
	}

	hasMore := len(teams) > pageSize
	if hasMore {
		teams = teams[:pageSize]
	}

	return teams, hasMore, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/daos/... -run TestTeamDao_SearchPublic -v`
Expected: PASS.

- [ ] **Step 5: Update the `mockTeamDao` in `cmd/api/services/team_service_test.go`**

Add the new method so the mock keeps satisfying `TeamDaoInterface` (needed before Task 9 compiles, but do it now to keep the package buildable):

```go
	searchPublicFn func(ctx *gin.Context, filters daos.TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error)
```

```go
func (m *mockTeamDao) SearchPublic(ctx *gin.Context, filters daos.TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error) {
	if m.searchPublicFn != nil {
		return m.searchPublicFn(ctx, filters, callerID, page, pageSize)
	}
	return nil, false, nil
}
```

Add `"simple-arq-golang/cmd/api/daos"` to that file's imports if not already present.

- [ ] **Step 6: Full build/vet**

Run: `go build ./... && go vet ./...`
Expected: green (this confirms every other `mockTeamDao`-implementing test file, if any exists outside `services`, still compiles — grep first: `grep -rln "mockTeamDao" cmd/api/**/*_test.go` to check for others; per current repo state there's only the one in `services`).

- [ ] **Step 7: Commit**

```bash
git add cmd/api/daos/team_dao.go cmd/api/daos/team_dao_test.go cmd/api/services/team_service_test.go
git commit -m "feat(daos): add TeamDao.SearchPublic"
```

---

### Task 5: DTOs — `domains/team` search types + `domains/joinrequest`

**Files:**
- Modify: `cmd/api/domains/team/team_update_request.go`
- Modify: `cmd/api/domains/team/team_response.go`
- Create: `cmd/api/domains/team/team_search.go`
- Create: `cmd/api/domains/joinrequest/join_request.go`

**Interfaces:**
- Produces: `team.SearchFilters`, `team.TeamSearchResult`, `team.TeamSearchResponse`; `joinrequest.JoinRequestResponse`, `joinrequest.PendingCountResponse` — used by Task 9 (`team_service.Search`) and Task 6/7/8 (`join_request_service`).

No test file for this task (pure structs, same precedent as the photos feature's DTO additions — validated by `go build` and exercised indirectly by the service/controller tests in later tasks).

- [ ] **Step 1: Extend `UpdateTeamRequest` and `TeamResponse`**

In `cmd/api/domains/team/team_update_request.go`, add:

```go
	Visible             *bool   `json:"visible"`   // Si aparece en resultados de búsqueda (opcional)
	IsPublic            *bool   `json:"is_public"` // Si acepta solicitudes de ingreso (opcional)
```

In `cmd/api/domains/team/team_response.go`, add:

```go
	Visible             bool      `json:"visible"`   // Si aparece en resultados de búsqueda
	IsPublic            bool      `json:"is_public"` // Si acepta solicitudes de ingreso
```

- [ ] **Step 2: Create `team_search.go`**

```go
package team

// SearchFilters agrupa los filtros opcionales de GET /api/v1/teams/search.
type SearchFilters struct {
	Name     string
	Level    string
	Country  string
	Province string
	City     string
}

// TeamSearchResult es una card de resultado de búsqueda de equipos.
type TeamSearchResult struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Level       string  `json:"level"`
	Country     string  `json:"country"`
	Province    string  `json:"province"`
	City        string  `json:"city"`
	MaxMembers  int64   `json:"max_members"`
	MemberCount int64   `json:"member_count"`
	OwnerName   string  `json:"owner_name"`
	IconURL     *string `json:"icon_url"`
	IsPublic    bool    `json:"is_public"`
}

// TeamSearchResponse es la respuesta paginada de GET /api/v1/teams/search.
type TeamSearchResponse struct {
	Teams   []TeamSearchResult `json:"teams"`
	HasMore bool               `json:"has_more"`
}
```

- [ ] **Step 3: Create `domains/joinrequest/join_request.go`**

```go
package joinrequest

import "time"

// JoinRequestResponse es el DTO de respuesta para una solicitud de ingreso.
type JoinRequestResponse struct {
	ID         int64     `json:"id"`
	TeamID     int64     `json:"team_id"`
	TeamName   string    `json:"team_name"`
	RunnerID   int64     `json:"runner_id"`
	RunnerName string    `json:"runner_name"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// PendingCountResponse es la respuesta de GET /api/v1/join-requests/pending-count.
type PendingCountResponse struct {
	Count int64 `json:"count"`
}
```

- [ ] **Step 4: Update `team_service.go`'s `toResponse` to include the new fields**

In `cmd/api/services/team_service.go`, `toResponse` (around line 495), add to the returned struct:

```go
		Visible:             t.Visible,
		IsPublic:            t.IsPublic,
```

- [ ] **Step 5: Update `team_service.Update` to apply the new optional fields**

In `Update` (around line 271-288), add:

```go
	if req.Visible != nil {
		teamDB.Visible = *req.Visible
	}
	if req.IsPublic != nil {
		teamDB.IsPublic = *req.IsPublic
	}
```

- [ ] **Step 6: Verify it compiles and existing tests still pass**

Run: `go build ./... && go test ./cmd/api/services/... -run TestTeamService -v`
Expected: green (existing `Update`/`toResponse` tests don't assert on `Visible`/`IsPublic` yet, so they should keep passing unchanged — if any test does exact-struct comparison on `TeamResponse` it'll need the two new zero-value fields added to its expected literal; fix inline if `go test` shows a diff).

- [ ] **Step 7: Commit**

```bash
git add cmd/api/domains/team/team_update_request.go cmd/api/domains/team/team_response.go cmd/api/domains/team/team_search.go cmd/api/domains/joinrequest/join_request.go cmd/api/services/team_service.go
git commit -m "feat(team): add visible/is_public fields and search/join-request DTOs"
```

---

### Task 6: `join_request_service` — `Create` and `Cancel`

**Files:**
- Create: `cmd/api/services/join_request_service.go`
- Create: `cmd/api/services/join_request_service_test.go`

**Interfaces:**
- Consumes: `daos.JoinRequestDaoInterface` (Task 3), `daos.TeamDaoInterface`/`daos.TeamUserDaoInterface`/`daos.UserDaoInterface` (existing), `domains/joinrequest` (Task 5).
- Produces: `JoinRequestServiceInterface{Create, Cancel, ...}` (this task implements `Create`/`Cancel`; `Accept`/`Reject`/`ListMine`/`ListByTeam`/`PendingCount` are stubbed to compile and filled in Task 7/8), sentinel errors `ErrTeamNotFound`, `ErrTeamNotPublic`, `ErrTeamFull`, `ErrAlreadyMember`, `ErrJoinRequestAlreadyPending`, `ErrJoinRequestNotFound`, `ErrJoinRequestForbidden`, `ErrJoinRequestNotPending` — used by Task 11 (`join_request_controller`).

- [ ] **Step 1: Write the failing tests for `Create` and `Cancel`**

```go
// cmd/api/services/join_request_service_test.go
package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockJoinRequestDao struct {
	createFn                   func(ctx *gin.Context, jr *dbs.JoinRequest) error
	findByIDFn                 func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error)
	findPendingByTeamAndUserFn func(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error)
	findPendingByTeamFn        func(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error)
	findByUserFn               func(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error)
	updateStatusFn             func(ctx *gin.Context, id int64, status string) error
	deleteFn                   func(ctx *gin.Context, id int64) error
	countPendingByOwnerFn      func(ctx *gin.Context, ownerID int64) (int64, error)
}

func (m *mockJoinRequestDao) Create(ctx *gin.Context, jr *dbs.JoinRequest) error {
	if m.createFn != nil {
		return m.createFn(ctx, jr)
	}
	jr.ID = 1
	return nil
}
func (m *mockJoinRequestDao) FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
	if m.findPendingByTeamAndUserFn != nil {
		return m.findPendingByTeamAndUserFn(ctx, teamID, runnerID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
	if m.findPendingByTeamFn != nil {
		return m.findPendingByTeamFn(ctx, teamID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
	if m.findByUserFn != nil {
		return m.findByUserFn(ctx, runnerID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) UpdateStatus(ctx *gin.Context, id int64, status string) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}
func (m *mockJoinRequestDao) Delete(ctx *gin.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m *mockJoinRequestDao) CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error) {
	if m.countPendingByOwnerFn != nil {
		return m.countPendingByOwnerFn(ctx, ownerID)
	}
	return 0, nil
}

func TestJoinRequestService_Create_Success(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 2, nil },
	}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	resp, err := svc.Create(nil, 5, 7)

	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.TeamID)
	assert.Equal(t, string(constants.InvitationStatusPending), resp.Status)
}

func TestJoinRequestService_Create_TeamNotFound(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestJoinRequestService_Create_TeamNotPublic(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: false}, nil
	}}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamNotPublic)
}

func TestJoinRequestService_Create_AlreadyMember(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
		return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
	}}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrAlreadyMember)
}

func TestJoinRequestService_Create_TeamFull(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 2}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 2, nil },
	}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamFull)
}

func TestJoinRequestService_Create_AlreadyPending(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
	}
	jrDao := &mockJoinRequestDao{findPendingByTeamAndUserFn: func(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: 1, TeamID: teamID, RunnerID: runnerID}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrJoinRequestAlreadyPending)
}

func TestJoinRequestService_Cancel_Success(t *testing.T) {
	deleted := false
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		deleteFn: func(ctx *gin.Context, id int64) error {
			deleted = true
			return nil
		},
	}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestJoinRequestService_Cancel_NotFound(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestNotFound)
}

func TestJoinRequestService_Cancel_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, RunnerID: 99, Status: string(constants.InvitationStatusPending)}, nil
	}}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_Cancel_NotPending(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, RunnerID: 7, Status: string(constants.InvitationStatusAccepted)}, nil
	}}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestNotPending)
}

var _ = errors.New // placeholder import guard removed once other tests in this file use errors directly
```

> Note: check whether `mockUserDao` already exists in the `services` package (it should — used by `team_service_test.go`/`user_service_test.go`). If its zero-value `FindByID` doesn't return `(nil, nil)` by default, adjust the mock literal in tests above with an explicit `findByIDFn`. Remove the placeholder `var _ = errors.New` line once Task 7/8's tests (which use `errors.New` in fixtures) land in this same file — it's only here to keep the `errors` import from going unused if you run this task in isolation.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/services/... -run TestJoinRequestService -v`
Expected: FAIL — `NewJoinRequestService` undefined.

- [ ] **Step 3: Implement `join_request_service.go` (Create, Cancel, and the interface with stubs for the rest)**

```go
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/joinrequest"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrTeamNotFound              = errors.New("equipo no encontrado")
	ErrTeamNotPublic             = errors.New("el equipo no acepta solicitudes de ingreso")
	ErrTeamFull                  = errors.New("el equipo alcanzó su cupo máximo")
	ErrAlreadyMember             = errors.New("el usuario ya pertenece a este equipo")
	ErrJoinRequestAlreadyPending = errors.New("ya existe una solicitud pendiente a este equipo")
	ErrJoinRequestNotFound       = errors.New("solicitud no encontrada")
	ErrJoinRequestForbidden      = errors.New("no autorizado")
	ErrJoinRequestNotPending     = errors.New("la solicitud ya fue resuelta")
)

// JoinRequestServiceInterface define las operaciones de negocio para
// solicitudes de ingreso de un corredor a un equipo.
type JoinRequestServiceInterface interface {
	Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error)
	Cancel(ctx *gin.Context, requestID, callerID int64) error
	Accept(ctx *gin.Context, requestID, callerID int64) error
	Reject(ctx *gin.Context, requestID, callerID int64) error
	ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error)
	ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error)
	PendingCount(ctx *gin.Context, ownerID int64) (int64, error)
}

type joinRequestService struct {
	joinRequestDao daos.JoinRequestDaoInterface
	teamDao        daos.TeamDaoInterface
	teamUserDao    daos.TeamUserDaoInterface
	userDao        daos.UserDaoInterface
	groupDao       daos.GroupDaoInterface
	groupUserDao   daos.GroupUserDaoInterface
	installDao     daos.InstallmentDaoInterface
	db             *gorm.DB
}

// NewJoinRequestService crea una nueva instancia de JoinRequestService.
func NewJoinRequestService(
	joinRequestDao daos.JoinRequestDaoInterface,
	teamDao daos.TeamDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	userDao daos.UserDaoInterface,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	installDao daos.InstallmentDaoInterface,
	db *gorm.DB,
) JoinRequestServiceInterface {
	return &joinRequestService{
		joinRequestDao: joinRequestDao,
		teamDao:        teamDao,
		teamUserDao:    teamUserDao,
		userDao:        userDao,
		groupDao:       groupDao,
		groupUserDao:   groupUserDao,
		installDao:     installDao,
		db:             db,
	}
}

// Create crea una solicitud de ingreso pending. Valida que el equipo exista,
// sea público, tenga cupo, y que el caller no sea ya miembro ni tenga otra
// solicitud pendiente al mismo equipo.
func (s *joinRequestService) Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if teamDB == nil {
		return nil, ErrTeamNotFound
	}
	if !teamDB.IsPublic {
		return nil, ErrTeamNotPublic
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if existingMember != nil {
		return nil, ErrAlreadyMember
	}

	count, err := s.teamUserDao.CountActiveByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error counting team members for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if count >= teamDB.MaxMembers {
		return nil, ErrTeamFull
	}

	existingPending, err := s.joinRequestDao.FindPendingByTeamAndUser(ctx, teamID, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking duplicate join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if existingPending != nil {
		return nil, ErrJoinRequestAlreadyPending
	}

	jr := &dbs.JoinRequest{
		TeamID:   teamID,
		RunnerID: runnerID,
		Status:   string(constants.InvitationStatusPending),
	}
	if err := s.joinRequestDao.Create(ctx, jr); err != nil {
		customlogger.Error(ctx, "error creating join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}

	return s.toResponse(ctx, jr, teamDB.Name), nil
}

// Cancel borra la solicitud (hard delete, no hay estado "cancelled" — D1) si
// el caller es su dueño y sigue pending.
func (s *joinRequestService) Cancel(ctx *gin.Context, requestID, callerID int64) error {
	jr, err := s.joinRequestDao.FindByID(ctx, requestID)
	if err != nil {
		customlogger.Error(ctx, "error finding join request for cancel", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Cancel"))
		return fmt.Errorf("error al cancelar solicitud")
	}
	if jr == nil {
		return ErrJoinRequestNotFound
	}
	if jr.RunnerID != callerID {
		return ErrJoinRequestForbidden
	}
	if jr.Status != string(constants.InvitationStatusPending) {
		return ErrJoinRequestNotPending
	}

	if err := s.joinRequestDao.Delete(ctx, requestID); err != nil {
		customlogger.Error(ctx, "error deleting join request on cancel", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Cancel"))
		return fmt.Errorf("error al cancelar solicitud")
	}
	return nil
}

// Accept, Reject, ListMine, ListByTeam, PendingCount: implemented in Task 7/8.
func (s *joinRequestService) Accept(ctx *gin.Context, requestID, callerID int64) error {
	return fmt.Errorf("not implemented")
}
func (s *joinRequestService) Reject(ctx *gin.Context, requestID, callerID int64) error {
	return fmt.Errorf("not implemented")
}
func (s *joinRequestService) ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *joinRequestService) ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *joinRequestService) PendingCount(ctx *gin.Context, ownerID int64) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

// toResponse convierte un dbs.JoinRequest a su DTO de respuesta, resolviendo
// el nombre del corredor.
func (s *joinRequestService) toResponse(ctx *gin.Context, jr *dbs.JoinRequest, teamName string) *joinrequest.JoinRequestResponse {
	runnerName := ""
	if runner, err := s.userDao.FindByID(ctx, jr.RunnerID); err == nil && runner != nil {
		runnerName = runner.Name + " " + runner.Surname
	}
	return &joinrequest.JoinRequestResponse{
		ID:         jr.ID,
		TeamID:     jr.TeamID,
		TeamName:   teamName,
		RunnerID:   jr.RunnerID,
		RunnerName: runnerName,
		Status:     jr.Status,
		CreatedAt:  jr.CreatedAt,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/services/... -run TestJoinRequestService -v`
Expected: PASS (all `Create`/`Cancel` scenarios).

- [ ] **Step 5: Full build/vet**

Run: `go build ./... && go vet ./...`
Expected: green (`time` import in this file is unused until Task 7 adds `Accept` — remove it from the import block for now if `go vet` flags it, re-add in Task 7).

- [ ] **Step 6: Commit**

```bash
git add cmd/api/services/join_request_service.go cmd/api/services/join_request_service_test.go
git commit -m "feat(services): add JoinRequestService Create/Cancel"
```

---

### Task 7: `join_request_service` — `Accept` and `Reject`

**Files:**
- Modify: `cmd/api/services/join_request_service.go`
- Modify: `cmd/api/services/join_request_service_test.go`

**Interfaces:**
- Consumes: `ApplyTeamMembershipGate` (existing, `cmd/api/services/team_membership_gate.go`), `AssignToDefaultGroup` (Task 2).
- Produces: `JoinRequestServiceInterface.Accept`/`Reject` (real implementations, replacing the Task 6 stubs).

- [ ] **Step 1: Write the failing tests**

```go
// append to cmd/api/services/join_request_service_test.go

func TestJoinRequestService_Accept_FreeTeam_CreatesActiveMembership(t *testing.T) {
	var createdTeamUser *dbs.TeamUser
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error {
			assert.Equal(t, string(constants.InvitationStatusAccepted), status)
			return nil
		},
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 10, MembershipFee: 0}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			createdTeamUser = tu
			return nil
		},
	}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	require.NoError(t, err)
	require.NotNil(t, createdTeamUser)
	assert.Equal(t, string(constants.SubscriptionStatusActive), createdTeamUser.SubscriptionStatus)
}

func TestJoinRequestService_Accept_AlreadyMember_SkipsGateStillMarksAccepted(t *testing.T) {
	gateCreateCalled := false
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error { return nil },
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			gateCreateCalled = true
			return nil
		},
	}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	require.NoError(t, err)
	assert.False(t, gateCreateCalled)
}

func TestJoinRequestService_Accept_TeamFull(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 1}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
	}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrTeamFull)
}

func TestJoinRequestService_Accept_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 99}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_Accept_NotPending(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusAccepted)}, nil
	}}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestNotPending)
}

func TestJoinRequestService_Reject_Success(t *testing.T) {
	statusSet := ""
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error {
			statusSet = status
			return nil
		},
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Reject(nil, 1, 1)

	require.NoError(t, err)
	assert.Equal(t, string(constants.InvitationStatusRejected), statusSet)
}

func TestJoinRequestService_Reject_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 99}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Reject(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/services/... -run "TestJoinRequestService_Accept|TestJoinRequestService_Reject" -v`
Expected: FAIL (`not implemented` errors from the Task 6 stubs).

- [ ] **Step 3: Implement `Accept`/`Reject`, replacing the Task 6 stubs**

Add `"time"` back to the imports, then replace the two stub functions:

```go
// findPendingRequestForOwner carga una solicitud pending y valida que callerID
// sea el entrenador dueño del equipo al que pertenece. Compartido por Accept/Reject.
func (s *joinRequestService) findPendingRequestForOwner(ctx *gin.Context, requestID, callerID int64, method string) (*dbs.JoinRequest, *dbs.Team, error) {
	jr, err := s.joinRequestDao.FindByID(ctx, requestID)
	if err != nil {
		customlogger.Error(ctx, "error finding join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod(method))
		return nil, nil, fmt.Errorf("error al procesar solicitud")
	}
	if jr == nil {
		return nil, nil, ErrJoinRequestNotFound
	}
	if jr.Status != string(constants.InvitationStatusPending) {
		return nil, nil, ErrJoinRequestNotPending
	}

	teamDB, err := s.teamDao.FindByID(ctx, jr.TeamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod(method))
		return nil, nil, fmt.Errorf("error al procesar solicitud")
	}
	if teamDB == nil {
		return nil, nil, ErrTeamNotFound
	}
	if teamDB.OwnerID != callerID {
		return nil, nil, ErrJoinRequestForbidden
	}

	return jr, teamDB, nil
}

// Accept crea la membresía (gateada por membership_fee, mismo patrón secuencial
// que invitation_service.AcceptInvitation), asigna al grupo default, y marca la
// solicitud como accepted.
func (s *joinRequestService) Accept(ctx *gin.Context, requestID, callerID int64) error {
	jr, teamDB, err := s.findPendingRequestForOwner(ctx, requestID, callerID, "Accept")
	if err != nil {
		return err
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, teamDB.ID, jr.RunnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership on accept join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
		return fmt.Errorf("error al aceptar solicitud")
	}

	if existingMember == nil {
		count, err := s.teamUserDao.CountActiveByTeam(ctx, teamDB.ID)
		if err != nil {
			customlogger.Error(ctx, "error counting team members on accept", err,
				customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
			return fmt.Errorf("error al aceptar solicitud")
		}
		if count >= teamDB.MaxMembers {
			return ErrTeamFull
		}

		teamUser := &dbs.TeamUser{
			TeamID:         teamDB.ID,
			UserID:         jr.RunnerID,
			RoleInTeam:     string(constants.TeamUserRoleCorredor),
			Status:         "active",
			AssignmentDate: time.Now(),
		}
		if err := ApplyTeamMembershipGate(ctx, s.db, s.teamUserDao, s.installDao, teamUser, teamDB.MembershipFee); err != nil {
			customlogger.Error(ctx, "error creating team_user on accept join request", err,
				customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
			return fmt.Errorf("error al aceptar solicitud")
		}
	}

	AssignToDefaultGroup(ctx, s.groupDao, s.groupUserDao, teamDB.ID, nil, jr.RunnerID)

	if err := s.joinRequestDao.UpdateStatus(ctx, jr.ID, string(constants.InvitationStatusAccepted)); err != nil {
		customlogger.Error(ctx, "team_user creado pero join request no pudo marcarse como aceptada", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
		return fmt.Errorf("error al aceptar solicitud")
	}

	customlogger.Info(ctx, "join request accepted successfully",
		customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
	return nil
}

// Reject marca la solicitud como rejected sin crear ninguna membresía.
func (s *joinRequestService) Reject(ctx *gin.Context, requestID, callerID int64) error {
	jr, _, err := s.findPendingRequestForOwner(ctx, requestID, callerID, "Reject")
	if err != nil {
		return err
	}

	if err := s.joinRequestDao.UpdateStatus(ctx, jr.ID, string(constants.InvitationStatusRejected)); err != nil {
		customlogger.Error(ctx, "error rejecting join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Reject"))
		return fmt.Errorf("error al rechazar solicitud")
	}
	return nil
}
```

Delete the two old stub functions (`Accept`/`Reject` returning `"not implemented"`) from Task 6's version of the file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/services/... -run TestJoinRequestService -v`
Expected: PASS (all `Create`/`Cancel`/`Accept`/`Reject` scenarios).

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/services/join_request_service.go cmd/api/services/join_request_service_test.go
git commit -m "feat(services): add JoinRequestService Accept/Reject"
```

---

### Task 8: `join_request_service` — `ListMine`, `ListByTeam`, `PendingCount`

**Files:**
- Modify: `cmd/api/services/join_request_service.go`
- Modify: `cmd/api/services/join_request_service_test.go`

**Interfaces:**
- Produces: `JoinRequestServiceInterface.ListMine`/`ListByTeam`/`PendingCount` (real implementations).

- [ ] **Step 1: Write the failing tests**

```go
// append to cmd/api/services/join_request_service_test.go

func TestJoinRequestService_ListMine(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByUserFn: func(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
		return []dbs.JoinRequest{{ID: 1, TeamID: 5, RunnerID: runnerID, Status: string(constants.InvitationStatusPending)}}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, Name: "equipo test"}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	results, err := svc.ListMine(nil, 7)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "equipo test", results[0].TeamName)
}

func TestJoinRequestService_ListByTeam_Success(t *testing.T) {
	jrDao := &mockJoinRequestDao{findPendingByTeamFn: func(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
		return []dbs.JoinRequest{{ID: 1, TeamID: teamID, RunnerID: 7, Status: string(constants.InvitationStatusPending)}}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 1, Name: "equipo test"}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	results, err := svc.ListByTeam(nil, 5, 1)

	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestJoinRequestService_ListByTeam_NotOwner(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 99}, nil
	}}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.ListByTeam(nil, 5, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_PendingCount(t *testing.T) {
	jrDao := &mockJoinRequestDao{countPendingByOwnerFn: func(ctx *gin.Context, ownerID int64) (int64, error) {
		return 3, nil
	}}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, &mockUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	count, err := svc.PendingCount(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/services/... -run "TestJoinRequestService_ListMine|TestJoinRequestService_ListByTeam|TestJoinRequestService_PendingCount" -v`
Expected: FAIL (`not implemented`).

- [ ] **Step 3: Implement, replacing the remaining Task 6 stubs**

```go
// ListMine devuelve todas las solicitudes del corredor, cualquier estado.
func (s *joinRequestService) ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
	requests, err := s.joinRequestDao.FindByUser(ctx, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error listing join requests for user", err,
			customlogger.Tag("runner_id", fmt.Sprintf("%d", runnerID)), customlogger.TagMethod("ListMine"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}

	responses := make([]joinrequest.JoinRequestResponse, len(requests))
	for i, jr := range requests {
		teamName := ""
		if teamDB, err := s.teamDao.FindByID(ctx, jr.TeamID); err == nil && teamDB != nil {
			teamName = teamDB.Name
		}
		responses[i] = *s.toResponse(ctx, &jr, teamName)
	}
	return responses, nil
}

// ListByTeam devuelve las solicitudes pending de un equipo, solo para su
// entrenador dueño.
func (s *joinRequestService) ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request list", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("ListByTeam"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}
	if teamDB == nil {
		return nil, ErrTeamNotFound
	}
	if teamDB.OwnerID != callerID {
		return nil, ErrJoinRequestForbidden
	}

	requests, err := s.joinRequestDao.FindPendingByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error listing pending join requests for team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("ListByTeam"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}

	responses := make([]joinrequest.JoinRequestResponse, len(requests))
	for i, jr := range requests {
		responses[i] = *s.toResponse(ctx, &jr, teamDB.Name)
	}
	return responses, nil
}

// PendingCount suma las solicitudes pending en todos los equipos que
// administra ownerID, para el badge del entrenador.
func (s *joinRequestService) PendingCount(ctx *gin.Context, ownerID int64) (int64, error) {
	count, err := s.joinRequestDao.CountPendingByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error counting pending join requests", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)), customlogger.TagMethod("PendingCount"))
		return 0, fmt.Errorf("error al contar solicitudes")
	}
	return count, nil
}
```

Delete the remaining old stubs (`ListMine`/`ListByTeam`/`PendingCount` returning `"not implemented"`) from Task 6's version.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/services/... -run TestJoinRequestService -v`
Expected: PASS — full `JoinRequestServiceInterface` now implemented and tested.

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/services/join_request_service.go cmd/api/services/join_request_service_test.go
git commit -m "feat(services): add JoinRequestService ListMine/ListByTeam/PendingCount"
```

---

### Task 9: `team_service.Search`

**Files:**
- Modify: `cmd/api/services/team_service.go`
- Modify: `cmd/api/services/team_service_test.go`

**Interfaces:**
- Consumes: `daos.TeamDaoInterface.SearchPublic` (Task 4).
- Produces: `TeamServiceInterface.Search(ctx *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error)`, sentinel `ErrInvalidQuery` — used by Task 12 (`team_controller.Search`).

- [ ] **Step 1: Write the failing tests**

```go
// append to cmd/api/services/team_service_test.go

func TestTeamService_Search_Success(t *testing.T) {
	teamDao := &mockTeamDao{searchPublicFn: func(ctx *gin.Context, filters daos.TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error) {
		return []dbs.Team{{ID: 1, Name: "equipo test", OwnerID: 2, MaxMembers: 10}}, false, nil
	}}
	userDao := &mockUserDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.User, error) {
		return &dbs.User{ID: id, Name: "Ana", Surname: "Gómez"}, nil
	}}
	teamUserDao := &mockTeamUserDao{countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 3, nil }}
	svc := NewTeamService(teamDao, userDao, &mockUserRoleDao{}, &mockRoleDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{}, nil)

	resp, err := svc.Search(nil, 99, team.SearchFilters{}, 1)

	require.NoError(t, err)
	require.Len(t, resp.Teams, 1)
	assert.Equal(t, "Ana Gómez", resp.Teams[0].OwnerName)
	assert.Equal(t, int64(3), resp.Teams[0].MemberCount)
	assert.False(t, resp.HasMore)
}

func TestTeamService_Search_InvalidPage(t *testing.T) {
	svc := NewTeamService(&mockTeamDao{}, &mockUserDao{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{}, nil)

	_, err := svc.Search(nil, 99, team.SearchFilters{}, 0)

	assert.ErrorIs(t, err, ErrInvalidQuery)
}
```

Add `"simple-arq-golang/cmd/api/daos"` and `"simple-arq-golang/cmd/api/domains/team"` to this test file's imports if not already present (`team` should already be imported for other `TeamResponse` tests).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/services/... -run TestTeamService_Search -v`
Expected: FAIL — `Search`/`ErrInvalidQuery` undefined.

- [ ] **Step 3: Implement `Search`**

Add to `TeamServiceInterface` in `team_service.go`:

```go
	Search(ctx *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error)
```

Add near the top of the file, with the other package-level consts/vars:

```go
const teamSearchPageSize = 20

var ErrInvalidQuery = errors.New("page debe ser mayor o igual a 1")
```

(add `"errors"` to the import block).

Add the method:

```go
// Search busca equipos visible=true con filtros opcionales, paginado por
// página fija de teamSearchPageSize, excluyendo equipos donde callerID ya es
// miembro.
func (s *teamService) Search(ctx *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error) {
	if page < 1 {
		return nil, ErrInvalidQuery
	}

	daoFilters := daos.TeamSearchFilters{
		Name:     filters.Name,
		Level:    filters.Level,
		Country:  filters.Country,
		Province: filters.Province,
		City:     filters.City,
	}

	teams, hasMore, err := s.teamDao.SearchPublic(ctx, daoFilters, callerID, page, teamSearchPageSize)
	if err != nil {
		customlogger.Error(ctx, "error searching teams", err, customlogger.TagMethod("Search"))
		return nil, fmt.Errorf("error al buscar equipos")
	}

	results := make([]team.TeamSearchResult, len(teams))
	for i, t := range teams {
		ownerName := ""
		if owner, err := s.userDao.FindByID(ctx, t.OwnerID); err == nil && owner != nil {
			ownerName = owner.Name + " " + owner.Surname
		}

		memberCount, err := s.teamUserDao.CountActiveByTeam(ctx, t.ID)
		if err != nil {
			memberCount = 0
		}

		results[i] = team.TeamSearchResult{
			ID:          t.ID,
			Name:        t.Name,
			Level:       t.Level,
			Country:     t.Country,
			Province:    t.Province,
			City:        t.City,
			MaxMembers:  t.MaxMembers,
			MemberCount: memberCount,
			OwnerName:   ownerName,
			IconURL:     buildMediaURL(t.IconKey, t.IconUpdatedAt),
			IsPublic:    t.IsPublic,
		}
	}

	return &team.TeamSearchResponse{Teams: results, HasMore: hasMore}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/services/... -run TestTeamService_Search -v`
Expected: PASS.

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/services/team_service.go cmd/api/services/team_service_test.go
git commit -m "feat(services): add TeamService.Search"
```

---

### Task 10: `join_request_controller`

**Files:**
- Create: `cmd/api/controllers/join_request_controller.go`
- Create: `cmd/api/controllers/join_request_controller_test.go`

**Interfaces:**
- Consumes: `services.JoinRequestServiceInterface` (Task 6/7/8), `utils.GetAuthUserID` (existing).
- Produces: `JoinRequestController{Create, Cancel, Accept, Reject, ListMine, ListByTeam, PendingCount}` — wired into routes in Task 13.

- [ ] **Step 1: Write the failing tests**

```go
// cmd/api/controllers/join_request_controller_test.go
package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/joinrequest"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockJoinRequestService struct {
	createFn       func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error)
	cancelFn       func(ctx *gin.Context, requestID, callerID int64) error
	acceptFn       func(ctx *gin.Context, requestID, callerID int64) error
	rejectFn       func(ctx *gin.Context, requestID, callerID int64) error
	listMineFn     func(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error)
	listByTeamFn   func(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error)
	pendingCountFn func(ctx *gin.Context, ownerID int64) (int64, error)
}

func (m *mockJoinRequestService) Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
	return m.createFn(ctx, teamID, runnerID)
}
func (m *mockJoinRequestService) Cancel(ctx *gin.Context, requestID, callerID int64) error {
	return m.cancelFn(ctx, requestID, callerID)
}
func (m *mockJoinRequestService) Accept(ctx *gin.Context, requestID, callerID int64) error {
	return m.acceptFn(ctx, requestID, callerID)
}
func (m *mockJoinRequestService) Reject(ctx *gin.Context, requestID, callerID int64) error {
	return m.rejectFn(ctx, requestID, callerID)
}
func (m *mockJoinRequestService) ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
	return m.listMineFn(ctx, runnerID)
}
func (m *mockJoinRequestService) ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
	return m.listByTeamFn(ctx, teamID, callerID)
}
func (m *mockJoinRequestService) PendingCount(ctx *gin.Context, ownerID int64) (int64, error) {
	return m.pendingCountFn(ctx, ownerID)
}

func newAuthedContext(method, path string, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set(utils.AuthUserIDKey, userID)
	return c, w
}

func TestJoinRequestController_Create_Success(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return &joinrequest.JoinRequestResponse{ID: 1, TeamID: teamID, RunnerID: runnerID}, nil
	}}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodPost, "/api/v1/teams/5/join-requests", 7)
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	ctrl.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestJoinRequestController_Create_TeamNotPublic(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrTeamNotPublic
	}}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodPost, "/api/v1/teams/5/join-requests", 7)
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	ctrl.Create(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_Create_TeamFull(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrTeamFull
	}}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodPost, "/api/v1/teams/5/join-requests", 7)
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	ctrl.Create(c)

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "TEAM_FULL")
}

func TestJoinRequestController_Cancel_NotFound(t *testing.T) {
	svc := &mockJoinRequestService{cancelFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestNotFound
	}}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodDelete, "/api/v1/join-requests/1", 7)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	ctrl.Cancel(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestJoinRequestController_Accept_Success(t *testing.T) {
	svc := &mockJoinRequestService{acceptFn: func(ctx *gin.Context, requestID, callerID int64) error { return nil }}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodPost, "/api/v1/join-requests/1/accept", 1)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	ctrl.Accept(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJoinRequestController_Accept_Forbidden(t *testing.T) {
	svc := &mockJoinRequestService{acceptFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestForbidden
	}}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodPost, "/api/v1/join-requests/1/accept", 2)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	ctrl.Accept(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_PendingCount(t *testing.T) {
	svc := &mockJoinRequestService{pendingCountFn: func(ctx *gin.Context, ownerID int64) (int64, error) { return 4, nil }}
	ctrl := NewJoinRequestController(svc)
	c, w := newAuthedContext(http.MethodGet, "/api/v1/join-requests/pending-count", 1)

	ctrl.PendingCount(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"count":4`)
}
```

> Check `utils.AuthUserIDKey` is exported (it's used as `c.Get(AuthUserIDKey)` inside `utils.GetAuthUserID` — confirm the constant itself, not just the function, is exported before using it directly in tests; if it isn't, set the context value the same way an existing controller test in this package already does and copy that helper instead of inventing `newAuthedContext`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/controllers/... -run TestJoinRequestController -v`
Expected: FAIL — `NewJoinRequestController` undefined.

- [ ] **Step 3: Implement `join_request_controller.go`**

```go
package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// JoinRequestController define las operaciones HTTP para solicitudes de ingreso.
type JoinRequestController interface {
	Create(c *gin.Context)
	Cancel(c *gin.Context)
	Accept(c *gin.Context)
	Reject(c *gin.Context)
	ListMine(c *gin.Context)
	ListByTeam(c *gin.Context)
	PendingCount(c *gin.Context)
}

type joinRequestController struct {
	joinRequestService services.JoinRequestServiceInterface
}

// NewJoinRequestController crea una nueva instancia de JoinRequestController.
func NewJoinRequestController(joinRequestService services.JoinRequestServiceInterface) JoinRequestController {
	return &joinRequestController{joinRequestService: joinRequestService}
}

// mapJoinRequestError traduce los sentinels de services a (status, code) HTTP.
func mapJoinRequestError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrTeamNotFound):
		return http.StatusNotFound, "TEAM_NOT_FOUND"
	case errors.Is(err, services.ErrTeamNotPublic):
		return http.StatusForbidden, "TEAM_NOT_PUBLIC"
	case errors.Is(err, services.ErrTeamFull):
		return http.StatusConflict, "TEAM_FULL"
	case errors.Is(err, services.ErrAlreadyMember):
		return http.StatusConflict, "ALREADY_MEMBER"
	case errors.Is(err, services.ErrJoinRequestAlreadyPending):
		return http.StatusConflict, "JOIN_REQUEST_ALREADY_PENDING"
	case errors.Is(err, services.ErrJoinRequestNotFound):
		return http.StatusNotFound, "JOIN_REQUEST_NOT_FOUND"
	case errors.Is(err, services.ErrJoinRequestForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, services.ErrJoinRequestNotPending):
		return http.StatusConflict, "JOIN_REQUEST_NOT_PENDING"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}

func respondJoinRequestError(c *gin.Context, err error) {
	statusCode, code := mapJoinRequestError(err)
	c.JSON(statusCode, apierror.APIError{StatusCode: statusCode, Code: code, Message: err.Error()})
}

// Create godoc
// @Summary      Solicitar ingreso a un equipo
// @Tags         join-requests
// @Produce      json
// @Param        id  path      int  true  "Team ID"
// @Success      201  {object}  joinrequest.JoinRequestResponse
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/join-requests [post]
func (jc *joinRequestController) Create(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "team id debe ser un número válido"})
		return
	}

	runnerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.Create(c, teamID, runnerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Cancel godoc
// @Summary      Cancelar solicitud propia
// @Tags         join-requests
// @Param        id  path  int  true  "Join request ID"
// @Success      204
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id} [delete]
func (jc *joinRequestController) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Cancel(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Accept godoc
// @Summary      Aceptar solicitud de ingreso
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Join request ID"
// @Success      200
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id}/accept [post]
func (jc *joinRequestController) Accept(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Accept(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitud aceptada"})
}

// Reject godoc
// @Summary      Rechazar solicitud de ingreso
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Join request ID"
// @Success      200
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id}/reject [post]
func (jc *joinRequestController) Reject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Reject(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitud rechazada"})
}

// ListMine godoc
// @Summary      Mis solicitudes de ingreso
// @Tags         join-requests
// @Produce      json
// @Success      200  {array}  joinrequest.JoinRequestResponse
// @Router       /api/v1/join-requests/mine [get]
func (jc *joinRequestController) ListMine(c *gin.Context) {
	runnerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.ListMine(c, runnerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListByTeam godoc
// @Summary      Solicitudes pendientes de un equipo
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Team ID"
// @Success      200  {array}  joinrequest.JoinRequestResponse
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/join-requests [get]
func (jc *joinRequestController) ListByTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "team id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.ListByTeam(c, teamID, callerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// PendingCount godoc
// @Summary      Conteo agregado de solicitudes pendientes
// @Tags         join-requests
// @Produce      json
// @Success      200  {object}  joinrequest.PendingCountResponse
// @Router       /api/v1/join-requests/pending-count [get]
func (jc *joinRequestController) PendingCount(c *gin.Context) {
	ownerID, _ := utils.GetAuthUserID(c)
	count, err := jc.joinRequestService.PendingCount(c, ownerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/controllers/... -run TestJoinRequestController -v`
Expected: PASS.

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/controllers/join_request_controller.go cmd/api/controllers/join_request_controller_test.go
git commit -m "feat(controllers): add JoinRequestController"
```

---

### Task 11: `team_controller.Search`

**Files:**
- Modify: `cmd/api/controllers/team_controller.go`
- Modify: `cmd/api/controllers/team_controller_test.go`

**Interfaces:**
- Consumes: `services.TeamServiceInterface.Search` (Task 9).
- Produces: `TeamController.Search(c *gin.Context)` — wired into routes in Task 13.

- [ ] **Step 1: Write the failing tests**

```go
// append to cmd/api/controllers/team_controller_test.go

func TestTeamController_Search_Success(t *testing.T) {
	svc := &mockTeamService{searchFn: func(c *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error) {
		assert.Equal(t, "medio", filters.Level)
		assert.Equal(t, 2, page)
		return &team.TeamSearchResponse{Teams: []team.TeamSearchResult{{ID: 1}}, HasMore: false}, nil
	}}
	ctrl := NewTeamController(svc, nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teams/search?level=medio&page=2", nil)

	ctrl.Search(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTeamController_Search_InvalidPage(t *testing.T) {
	svc := &mockTeamService{searchFn: func(c *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error) {
		return nil, services.ErrInvalidQuery
	}}
	ctrl := NewTeamController(svc, nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teams/search?page=0", nil)

	ctrl.Search(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_QUERY")
}
```

> Check this test file's existing `mockTeamService` struct (used by other `team_controller_test.go` tests) and add a `searchFn func(c *gin.Context, callerID int64, filters team.SearchFilters, page int) (*team.TeamSearchResponse, error)` field plus the matching `Search` method, following the same `if m.searchFn != nil { ... }` pattern as its sibling methods.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/api/controllers/... -run TestTeamController_Search -v`
Expected: FAIL — `Search` undefined on `teamController`.

- [ ] **Step 3: Implement `Search`**

Add `Search(c *gin.Context)` to the `TeamController` interface in `team_controller.go`, then:

```go
// Search godoc
// @Summary      Buscar equipos
// @Description  Busca equipos visible=true por nombre/nivel/ubicación, excluyendo equipos donde el caller ya es miembro. Paginado (page, tamaño fijo 20)
// @Tags         teams
// @Produce      json
// @Param        name      query     string  false  "Nombre (parcial)"
// @Param        level     query     string  false  "Nivel"
// @Param        country   query     string  false  "País"
// @Param        province  query     string  false  "Provincia"
// @Param        city      query     string  false  "Ciudad"
// @Param        page      query     int     false  "Página (default 1)"
// @Success      200  {object}  team.TeamSearchResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams/search [get]
func (tc *teamController) Search(c *gin.Context) {
	page := 1
	if p := c.Query("page"); p != "" {
		parsed, err := strconv.Atoi(p)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "INVALID_QUERY", Message: "page debe ser un número válido"})
			return
		}
		page = parsed
	}

	filters := team.SearchFilters{
		Name:     c.Query("name"),
		Level:    c.Query("level"),
		Country:  c.Query("country"),
		Province: c.Query("province"),
		City:     c.Query("city"),
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := tc.teamService.Search(c, callerID, filters, page)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		if errors.Is(err, services.ErrInvalidQuery) {
			statusCode = http.StatusBadRequest
			code = "INVALID_QUERY"
		}
		c.JSON(statusCode, apierror.APIError{StatusCode: statusCode, Code: code, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/api/controllers/... -run TestTeamController_Search -v`
Expected: PASS.

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/controllers/team_controller.go cmd/api/controllers/team_controller_test.go
git commit -m "feat(controllers): add TeamController.Search"
```

---

### Task 12: Wire dependency injection and routes

**Files:**
- Modify: `cmd/api/app/app.go`
- Modify: `cmd/api/app/url_mappings.go`

**Interfaces:**
- Consumes: everything from Tasks 1-11.
- Produces: the 8 live HTTP routes.

- [ ] **Step 1: Wire the new DAO/service/controller in `app.go`**

Near the existing `teamUserDao`/`invitationDao`/`invitationService` wiring (around lines 165-203), add:

```go
	joinRequestDao := daos.NewJoinRequestDao(db)
```

Right after `teamService := services.NewTeamService(...)` (so `teamService` already exists), the `teamService` itself doesn't need new constructor args (its interface just grew a method) — no change needed there. Add after `invitationController := controllers.NewInvitationController(invitationService)`:

```go
	joinRequestService := services.NewJoinRequestService(joinRequestDao, teamDao, teamUserDao, userDao, groupDao, groupUserDao, installmentDao, db)
	joinRequestController := controllers.NewJoinRequestController(joinRequestService)
```

Add `joinRequestController` (and, if this repo's `App` struct stores controllers as named fields rather than closures — check how `invitationController` is stored/referenced, e.g. `app.invitationController` in `url_mappings.go` implies a struct field) to whatever struct holds `app.teamController`/`app.invitationController`, following that exact same pattern.

- [ ] **Step 2: Register the 8 routes in `url_mappings.go`**

Add `r.GET("/api/v1/teams/search", app.teamController.Search)` **before** the block of `/api/v1/teams/:id...` routes (readability — gin's router resolves static vs. `:id` routes correctly regardless of registration order, but keeping the static route visually first avoids confusion for the next person reading this file).

Add a new `// Join Requests` section, near the existing `// Invitations` block:

```go
	// Join Requests
	r.GET("/api/v1/teams/:id/join-requests", app.joinRequestController.ListByTeam)
	r.POST("/api/v1/teams/:id/join-requests", app.joinRequestController.Create)
	r.GET("/api/v1/join-requests/mine", app.joinRequestController.ListMine)
	r.GET("/api/v1/join-requests/pending-count", app.joinRequestController.PendingCount)
	r.DELETE("/api/v1/join-requests/:id", app.joinRequestController.Cancel)
	r.POST("/api/v1/join-requests/:id/accept", app.joinRequestController.Accept)
	r.POST("/api/v1/join-requests/:id/reject", app.joinRequestController.Reject)
```

(`/api/v1/join-requests/mine` and `/api/v1/join-requests/pending-count` must be registered — gin handles static-vs-`:id` at the same segment level fine, but list them before `:id`-based routes in the file for the same readability reason as `teams/search`.)

- [ ] **Step 3: Full build**

Run: `go build ./...`
Expected: green — this is the step that catches any DI wiring mistake (wrong arg order/count).

- [ ] **Step 4: Run the whole test suite**

Run: `go test ./...`
Expected: green.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/app/app.go cmd/api/app/url_mappings.go
git commit -m "feat(app): wire join-requests and team search routes"
```

---

### Task 13: Swagger regeneration and README

**Files:**
- Modify: `cmd/api/docs/docs.go`, `cmd/api/docs/swagger.json`, `cmd/api/docs/swagger.yaml` (generated)
- Modify: `README.md`

- [ ] **Step 1: Regenerate swagger**

Run: `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` (per `.agentics/WORKFLOW.md` — install `swag` first if missing, matching how the photos feature did it: `go install github.com/swaggo/swag/cmd/swag@<version matching go.mod>`).

- [ ] **Step 2: Confirm the new paths are present**

Run: `grep -c "join-requests\|teams/search" cmd/api/docs/swagger.json`
Expected: non-zero.

- [ ] **Step 3: Update the endpoints table in `README.md`**

Add rows for the 8 new endpoints (path, method, auth requirement) in the same table format already used for the other endpoint groups.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/docs/ README.md
git commit -m "docs: regenerate swagger and README for team search/join-requests"
```

---

### Task 14: Final verification

**Files:** none (verification only).

- [ ] **Step 1: Full suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all green.

- [ ] **Step 2: Coverage sanity check**

Run: `make coverage` (or `make coverage-with-db` if `TEST_DB_HOST` is available locally — see `docs/TESTING.md`)
Expected: no package with real logic and 0% coverage among the new files (`join_request_dao.go`, `join_request_service.go`, `join_request_controller.go`, `team_group_assignment.go`, `team_dao.go`'s `SearchPublic`, `team_service.go`'s `Search`, `team_controller.go`'s `Search` should all show meaningful coverage from the tests written in Tasks 3-11).

- [ ] **Step 3: Manual end-to-end check against the real testing DB (optional but recommended before opening the PR)**

Using Bruno or `curl` against a locally running server pointed at the testing Supabase DB (default stage, no `--stage=production` flag):
1. Log in as an existing entrenador, create a team (defaults to `visible=true, is_public=true`).
2. Log in as a different corredor, `GET /api/v1/teams/search` — confirm the new team appears.
3. `POST /api/v1/teams/:id/join-requests` — confirm `201` with `status: "pending"`.
4. Back as the entrenador, `GET /api/v1/teams/:id/join-requests` — confirm the pending request appears; `GET /api/v1/join-requests/pending-count` — confirm it's counted.
5. `POST /api/v1/join-requests/:id/accept` — confirm `200`.
6. `GET /api/v1/teams/:id/users` (existing endpoint) — confirm the corredor is now a member, in the team's default group.
7. Repeat steps 2-3 with a second corredor and immediately `DELETE /api/v1/join-requests/:id` (self) — confirm `204` and that it no longer shows up in `GET /api/v1/join-requests/mine`.

- [ ] **Step 4: No commit for this task** — pure verification. If step 3 finds a real bug, go back to the relevant task, fix it there (TDD: failing test first), and commit under that task's scope.

---

## Self-Review Notes

- **Spec coverage:** `team-search/spec.md`'s 3 requirements → Tasks 4, 9, 11. `team-join-requests/spec.md`'s 6 requirements → Tasks 3, 6, 7, 8, 10. D5 (`AssignToDefaultGroup`) → Task 2. D1 (schema) → Task 1. D7 (error codes) → Task 10's `mapJoinRequestError` + Task 11's `ErrInvalidQuery` handling. D6 (race condition, not fixed) → intentionally absent from every task, matches Non-Goals.
- **Placeholder scan:** none found — every step has real code or a concrete shell command.
- **Type consistency:** `JoinRequestServiceInterface` signatures introduced in Task 6 are used identically in Tasks 7, 8, 10 (`Accept(ctx, requestID, callerID int64) error`, etc.) — no renames across tasks. `daos.TeamSearchFilters` (Task 4) and `team.SearchFilters` (Task 5) are deliberately two different types in two different packages (DAO-layer vs. domain-layer), mapped explicitly in `team_service.Search` (Task 9) — not a naming inconsistency.
