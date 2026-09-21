package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// exclOwnerGroup crea dueño+team+grupo con tag único (patrón task3OwnerGroup).
func exclOwnerGroup(t *testing.T, db *gorm.DB, tag string) (*dbs.User, *dbs.Group) {
	t.Helper()
	owner := &dbs.User{Name: "Excl", Surname: tag, Email: "excl-" + tag + "@test.com", DNI: "5199" + tag}
	owner.BirthDate = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	owner.Password = "hashed"
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Excl " + tag + " team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Excl " + tag + " group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return owner, group
}

func exclSvc(db *gorm.DB) CalendarServiceInterface {
	return NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, db,
	)
}

func exclPlan(t *testing.T, db *gorm.DB, ownerID int64, name string, kinds []string, sessionID *int64) *dbs.TrainingPlan {
	t.Helper()
	plan := &dbs.TrainingPlan{OwnerID: ownerID, Name: name}
	require.NoError(t, db.Create(plan).Error)
	for i, k := range kinds {
		pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: i + 1, Kind: k}
		if k == "training" {
			pd.SessionID = sessionID
		}
		require.NoError(t, db.Create(&pd).Error)
	}
	return plan
}

func exclSession(t *testing.T, db *gorm.DB, ownerID int64, name string) *int64 {
	t.Helper()
	ex := &dbs.Exercise{OwnerID: ownerID, Name: name + " ex", Kind: "running"}
	require.NoError(t, db.Create(ex).Error)
	s := &dbs.Session{OwnerID: ownerID, Name: name + " session"}
	require.NoError(t, db.Create(s).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: s.ID, ExerciseID: ex.ID, Role: "main"}).Error)
	return &s.ID
}

// Task 3.1 — stamp con exclude_dates sobre un día ocupado conserva su
// instancia intacta (mismo session_instance_id) y la respuesta no lo incluye.
func TestStampExcludeDates_ConservaInstanciaExcluidaYRespuestaLaOmite(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "keepinst")
	sid := exclSession(t, db, owner.ID, "keep")
	plan := exclPlan(t, db, owner.ID, "plan keep", []string{"training", "training", "training"}, sid)
	start := time.Now().AddDate(0, 0, 3)
	startStr := start.Format("2006-01-02")
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := exclSvc(db)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startStr})
	require.NoError(t, err)

	// Estado del día 2 (a excluir) tras el primer stamp.
	day2 := start.AddDate(0, 0, 1)
	before, err := calendarDao.FindByGroupAndDate(nil, group.ID, day2)
	require.NoError(t, err)
	require.NotNil(t, before)
	require.NotNil(t, before.SessionInstanceID)
	var instBefore int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instBefore).Error)

	// Re-stamp con force excluyendo el día 2.
	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr, Force: true,
		ExcludeDates: []string{day2.Format("2006-01-02")},
	})
	require.NoError(t, err)

	require.Len(t, resp, 2, "la respuesta no incluye la fecha excluida")
	for _, r := range resp {
		assert.NotEqual(t, day2.Format("2006-01-02"), r.Date)
	}

	after, err := calendarDao.FindByGroupAndDate(nil, group.ID, day2)
	require.NoError(t, err)
	require.NotNil(t, after)
	require.NotNil(t, after.SessionInstanceID)
	assert.Equal(t, *before.SessionInstanceID, *after.SessionInstanceID, "el día excluido conserva su instancia")

	// Días 1 y 3 se re-instancian (2 creadas + 2 viejas borradas), día 2 intacto:
	// el total de SessionInstances no cambia.
	var instAfter int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instAfter).Error)
	assert.Equal(t, instBefore, instAfter)
}

// Task 3.2 — un día excluido con conflicto preexistente NO cuenta para el 409
// (force=false) y un día excluido cerrado NO cuenta para el 422.
func TestStampExcludeDates_FechasExcluidasNoDisparanGuards(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "noguards")
	plan := exclPlan(t, db, owner.ID, "plan noguards", []string{"rest", "rest"}, nil)
	start := time.Now().AddDate(0, 0, 3)
	startStr := start.Format("2006-01-02")
	day1 := start.AddDate(0, 0, 1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := exclSvc(db)

	// Conflicto: el día 2 (start+1) ya tiene contenido escrito directo por DAO.
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: day1, Kind: "rest"}))

	// force=false con el día en conflicto excluido → no hay 409, solo se estampa el día 1.
	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr,
		ExcludeDates: []string{day1.Format("2006-01-02")},
	})
	require.NoError(t, err, "el día excluido con contenido no cuenta para el 409")
	require.Len(t, resp, 1)
	assert.Equal(t, startStr, resp[0].Date)

	// Regresión: sin excluir ese día, el mismo stamp daría 409 (guard intacto).
	conflicting, err := calendarDao.FindByGroupAndDate(nil, group.ID, day1)
	require.NoError(t, err)
	require.NotNil(t, conflicting, "el día excluido quedó intacto")

	_, err = svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startStr, ExcludeDates: []string{}})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarStampConflict)
}

func TestStampExcludeDates_DiaCerradoExcluidoNoDispara422(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "closedex")
	plan := exclPlan(t, db, owner.ID, "plan closedex", []string{"rest", "rest", "rest"}, nil)
	// start = ayer → día 0 ayer (cerrado), día 1 hoy (cerrado), día 2 mañana (abierto).
	start := time.Now().AddDate(0, 0, -1)
	startStr := start.Format("2006-01-02")
	svc := exclSvc(db)

	// Base: sin exclusión, los días pasados disparan 422 (comportamiento intacto).
	_, errBase := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startStr, Force: true})
	require.Error(t, errBase)
	assert.ErrorIs(t, errBase, ErrCalendarDayClosed)

	// Excluyendo ayer y hoy: solo se estampa mañana, sin 422.
	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr, Force: true,
		ExcludeDates: []string{startStr, time.Now().Format("2006-01-02")},
	})
	require.NoError(t, err, "los días cerrados excluidos no cuentan para el 422")
	require.Len(t, resp, 1)
	assert.Equal(t, time.Now().AddDate(0, 0, 1).Format("2006-01-02"), resp[0].Date)
}

// Task 3.3 — exclude_dates con formato inválido → ErrCalendarInvalidDate sin escribir nada.
func TestStampExcludeDates_FormatoInvalidoRechazaSinEscribir(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "badfmt")
	plan := exclPlan(t, db, owner.ID, "plan badfmt", []string{"rest", "rest"}, nil)
	startStr := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	svc := exclSvc(db)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr, ExcludeDates: []string{"10/07/2026"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrCalendarInvalidDate)

	var dayCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ?", group.ID).Count(&dayCount).Error)
	assert.Zero(t, dayCount, "formato inválido aborta antes de escribir")
}

// Task 3.3 (cont.) — fecha excluida fuera del rango del plan se ignora.
func TestStampExcludeDates_FueraDeRangoSeIgnora(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "oor")
	plan := exclPlan(t, db, owner.ID, "plan oor", []string{"rest", "rest"}, nil)
	startStr := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	svc := exclSvc(db)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr, ExcludeDates: []string{"2999-01-01"},
	})
	require.NoError(t, err)
	require.Len(t, resp, 2, "una fecha fuera del rango no excluye ningún día del plan")
}

// Task 3.4 — rango totalmente excluido → respuesta [] (no-nil), sin escribir.
func TestStampExcludeDates_RangoVacioRespondeArrayVacio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "empty")
	plan := exclPlan(t, db, owner.ID, "plan empty", []string{"rest", "rest"}, nil)
	start := time.Now().AddDate(0, 0, 3)
	svc := exclSvc(db)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: start.Format("2006-01-02"),
		ExcludeDates: []string{start.Format("2006-01-02"), start.AddDate(0, 0, 1).Format("2006-01-02")},
	})
	require.NoError(t, err)
	require.NotNil(t, resp, "debe resolver a [] en JSON, no null")
	assert.Len(t, resp, 0)

	var dayCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ?", group.ID).Count(&dayCount).Error)
	assert.Zero(t, dayCount)
}

// Task 3.5 — regresión: omitir el campo o pasar [] estampa el rango completo.
func TestStampExcludeDates_SinCampoYArrayVacioIguales(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := exclOwnerGroup(t, db, "noregres")
	plan := exclPlan(t, db, owner.ID, "plan noregres", []string{"rest", "rest", "rest"}, nil)
	startStr := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	svc := exclSvc(db)

	respNil, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startStr})
	require.NoError(t, err)
	require.Len(t, respNil, 3)

	respEmpty, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{
		PlanID: plan.ID, StartDate: startStr, Force: true, ExcludeDates: []string{},
	})
	require.NoError(t, err)
	require.Len(t, respEmpty, 3, "exclude_dates:[] debe equivaler a omitir el campo")
}

// Cobertura del path mock (s.db == nil): formato inválido también aborta ahí.
func TestStampExcludeDates_MockFormatoInvalido(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return &dbs.Group{ID: id, TeamID: 1}, nil }}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return &dbs.Team{ID: id, OwnerID: 7}, nil }}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{SequenceNo: 1, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01", ExcludeDates: []string{"nope"}})
	assert.ErrorIs(t, err, ErrCalendarInvalidDate)
}
