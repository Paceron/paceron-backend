package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

// 1.1: con membresías en varios grupos, los nombres se resuelven solo para los
// group_id que aparecen en los días devueltos.
func TestCalendarService_NextSession_NamesOnlyForReturnedGroups(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}, {GroupID: 2, UserID: userID}, {GroupID: 3, UserID: userID}}, nil
	}}
	var lookedUp []int64
	groupDao := &mockGroupDao{findWithTeamNameByIDsFn: func(ctx *gin.Context, ids []int64) ([]daos.GroupWithTeamName, error) {
		lookedUp = append([]int64(nil), ids...)
		return []daos.GroupWithTeamName{{ID: 2, Name: "Grupo 2", TeamID: 10, TeamName: "Equipo 10"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindsFn: func(ctx *gin.Context, groupIDs []int64, kinds []string, today time.Time, nowHHMM string) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: 2, Date: today.AddDate(0, 0, 1), Kind: "training"}}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp.NextTraining)
	assert.Equal(t, []int64{2}, lookedUp, "solo se resuelven nombres de grupos devueltos")
	assert.Equal(t, "Grupo 2", resp.NextTraining.GroupName)
}

// 1.2: MemberCalendar resuelve nombres solo para los grupos con días en el
// rango, no para todos los grupos de las membresías.
func TestCalendarService_MemberCalendar_NamesOnlyForGroupsWithDays(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}, {GroupID: 2, UserID: userID}, {GroupID: 3, UserID: userID}}, nil
	}}
	var lookedUp []int64
	groupDao := &mockGroupDao{findWithTeamNameByIDsFn: func(ctx *gin.Context, ids []int64) ([]daos.GroupWithTeamName, error) {
		lookedUp = append([]int64(nil), ids...)
		return []daos.GroupWithTeamName{
			{ID: 1, Name: "Grupo 1", TeamID: 10, TeamName: "Equipo 10"},
			{ID: 3, Name: "Grupo 3", TeamID: 12, TeamName: "Equipo 12"},
		}, nil
	}}
	d1, _ := time.Parse("2006-01-02", "2099-10-05")
	d2, _ := time.Parse("2006-01-02", "2099-10-06")
	calDao := &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{
			{GroupID: 1, Date: d1, Kind: "training"},
			{GroupID: 3, Date: d2, Kind: "rest"},
		}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.MemberCalendar(nil, 42, time.Now(), time.Now().AddDate(0, 0, 7))

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, []int64{1, 3}, lookedUp, "el grupo sin días no se consulta")
	assert.Equal(t, "Grupo 1", resp[0].GroupName)
	assert.Equal(t, "Equipo 12", resp[1].TeamName)
}

// 1.3: NextPresencialSession resuelve el equipo solo del día seleccionado, no
// el de todos los grupos que administra el owner.
func TestCalendarService_NextPresencialSession_ResolvesOnlySelectedTeam(t *testing.T) {
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{
			{ID: 1, Name: "Grupo 1", TeamID: 10},
			{ID: 2, Name: "Grupo 2", TeamID: 11},
			{ID: 3, Name: "Grupo 3", TeamID: 12},
		}, nil
	}}
	var lookedUp []int64
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		lookedUp = append([]int64(nil), ids...)
		return []dbs.Team{{ID: 12, Name: "Equipo 12"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextPresencialFn: func(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: 3, Date: today.AddDate(0, 0, 1), Kind: "training", IsPresencial: true}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.NextPresencialSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []int64{12}, lookedUp, "solo el equipo del día seleccionado")
	assert.Equal(t, int64(3), resp.GroupID)
	assert.Equal(t, "Equipo 12", resp.TeamName)
}

// 1.4: UpsertDay lee el día una sola vez, dentro de la transacción.
func TestCalendarService_UpsertDay_SingleReadInTransaction(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Opt", Surname: "Owner", Email: "opt-upsert-owner@test.com", DNI: "57000101", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Opt team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Opt group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	date := time.Now().AddDate(0, 0, 42)
	counter := 0
	cdb := txCountingDB(t, db, &counter)
	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb),
		daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb,
	)

	resp, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "rest"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, date.Format("2006-01-02"), resp.Date)
	// 2 auth (grupo+equipo) + 1 FindByGroupAndDate en tx + 1 select + 1 write
	// del Upsert + 1 commit. Antes había un FindByGroupAndDate extra pre-tx.
	assert.Equal(t, 6, counter, "una sola lectura del día, dentro de la transacción")

	counter = 0
	resp, err = svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "rest"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 6, counter, "el update tampoco relectura fuera de la tx")
}

// DB path: la validación corre antes de buildRow, así que IsPresencial sin
// hora no panic-ea y una hora malformada conserva ErrCalendarInvalidTimeFormat
// (en vez del error genérico del wrap del buildRow).
func TestCalendarService_UpsertDay_DBPathPresencialValidationBeforeBuildRow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Opt", Surname: "Validate", Email: "opt-validate-owner@test.com", DNI: "57000102", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Opt validate team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Opt validate group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, nil, nil, nil, db,
	)
	isPresencial := true
	to := "19:30"
	location := &trainingplan.Location{Lat: -34.6, Lng: -58.4}

	t.Run("missing time", func(t *testing.T) {
		_, err := svc.UpsertDay(nil, group.ID, owner.ID, time.Now().AddDate(0, 0, 43), calendar.CalendarDayRequest{
			Kind: "rest", IsPresencial: &isPresencial, PresencialTimeTo: &to, PresencialLocation: location,
		})
		assert.ErrorIs(t, err, ErrCalendarFieldMismatch)
	})

	t.Run("malformed time", func(t *testing.T) {
		bad := "not-a-time"
		_, err := svc.UpsertDay(nil, group.ID, owner.ID, time.Now().AddDate(0, 0, 44), calendar.CalendarDayRequest{
			Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &bad, PresencialTimeTo: &to, PresencialLocation: location,
		})
		assert.ErrorIs(t, err, ErrCalendarInvalidTimeFormat)
	})

	day, err := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, time.Now().AddDate(0, 0, 43))
	require.NoError(t, err)
	assert.Nil(t, day, "la validación falla antes de escribir")
}
