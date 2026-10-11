package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/testutils"
)

// ---------------------------------------------------------------------------
// Task 6 (D10.1/D10.2): pins de seguridad.
//
// D10.1 SQLi: los filtros de texto cliente-facing (q de /users/search,
// country/province/city de /teams/search) llegan a la DB solo como parámetros
// de queries GORM. Estos pins corren contra Postgres real y garantizan que un
// payload SQLi (a) no produce error ni escala, y (b) matchea solo por literal
// si existe un registro cuyo dato ES el payload (probe de parameterización).
//
// D10.2 no-leak: con testutils.FailingDB (falla inyectada por tipo de
// operación SQL) se asegura que los mensajes de error que el service devuelve
// conservan el mensaje contractual, sin arrastrar texto del driver.
// ---------------------------------------------------------------------------

// sqlLeakTokens son marcadores típicos del texto de un error de driver PG.
// Si alguno aparece en un mensaje que termina en un body HTTP, hubo leak.
var sqlLeakTokens = []string{"SQLSTATE", "pgconn", "pq:", "syntax error", "Unterminated"}

// assertNoDriverLeak garantiza que el mensaje no arrastra texto del driver.
// Es casi redundante cuando el mismo test ya tiene EqualError sobre el mensaje
// contractual, pero queda como pin explícito del D10.2.
func assertNoDriverLeak(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	for _, tok := range sqlLeakTokens {
		assert.NotContains(t, err.Error(), tok)
	}
}

func sqlipersistUser(t *testing.T, db *gorm.DB, name, email, dni string) *dbs.User {
	t.Helper()
	u := &dbs.User{
		Name:      name,
		Surname:   "SQLi",
		Email:     email,
		DNI:       dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "hashed",
		Status:    "active",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

// --- D10.1: /users/search (q) ----------------------------------------------

func TestSQLi_UserSearch_PayloadsDoNotScale(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sqlipersistUser(t, db, "Ana", "sqli-payloads-ana@test.com", "71000001")
	svc := NewUserService(daos.NewUserDao(db), nil, nil, nil, nil)

	payloads := []string{"admin'+--", "' OR 1=1 --", "%; DROP TABLE users; --", "%  %"}
	for _, payload := range payloads {
		resp, err := svc.Search(nil, payload)
		require.NoError(t, err, payload)
		assert.Empty(t, resp.Results, payload)
	}
}

func TestSQLi_UserSearch_LiteralPayloadMatchedByLiteral(t *testing.T) {
	db := testutils.SetupTestDB(t)
	victim := sqlipersistUser(t, db, "' OR 1=1 --", "sqli-literal-victim@test.com", "71000002")
	svc := NewUserService(daos.NewUserDao(db), nil, nil, nil, nil)

	resp, err := svc.Search(nil, "' OR 1=1 --")

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, victim.ID, resp.Results[0].UserID)
}

// --- D10.1: /teams/search (country/province/city) ---------------------------

func TestSQLi_TeamSearch_PayloadFiltersDoNotReachSQL(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := sqlipersistUser(t, db, "Owner", "sqli-team-owner@test.com", "71000003")
	caller := sqlipersistUser(t, db, "Caller", "sqli-team-caller@test.com", "71000004")
	require.NoError(t, db.Create(&dbs.Team{
		Name: "Equipo Legítimo", MaxMembers: 10, OwnerID: owner.ID, Visible: true,
		Country: "Argentina", Province: "Buenos Aires", City: "CABA",
	}).Error)
	svc := NewTeamService(
		daos.NewTeamDao(db), daos.NewUserDao(db), nil, nil, daos.NewTeamUserDao(db),
		nil, nil, nil, nil, nil,
	)

	payloads := []string{"admin'+--", "' OR 1=1 --", "%; DROP TABLE teams; --"}
	for _, payload := range payloads {
		for _, tf := range []team.SearchFilters{
			{Country: payload},
			{Province: payload},
			{City: payload},
		} {
			resp, err := svc.Search(nil, caller.ID, tf, 1)
			require.NoError(t, err, payload)
			assert.Empty(t, resp.Teams, payload)
		}
	}
}

func TestSQLi_TeamSearch_LiteralPayloadStoredMatchesByEquality(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := sqlipersistUser(t, db, "Owner", "sqli-literal-owner@test.com", "71000005")
	caller := sqlipersistUser(t, db, "Caller", "sqli-literal-caller@test.com", "71000006")
	created := &dbs.Team{
		Name: "Equipo Literal", MaxMembers: 10, OwnerID: owner.ID, Visible: true,
		Country: "' OR 1=1 --", Province: "admin'+--", City: "%; DROP TABLE teams; --",
	}
	require.NoError(t, db.Create(created).Error)
	svc := NewTeamService(
		daos.NewTeamDao(db), daos.NewUserDao(db), nil, nil, daos.NewTeamUserDao(db),
		nil, nil, nil, nil, nil,
	)

	for _, tc := range []struct {
		name    string
		filters team.SearchFilters
	}{
		{"country", team.SearchFilters{Country: "' OR 1=1 --"}},
		{"province", team.SearchFilters{Province: "admin'+--"}},
		{"city", team.SearchFilters{City: "%; DROP TABLE teams; --"}},
	} {
		resp, err := svc.Search(nil, caller.ID, tc.filters, 1)
		require.NoError(t, err, tc.name)
		require.Len(t, resp.Teams, 1, tc.name)
		assert.Equal(t, created.ID, resp.Teams[0].ID, tc.name)
	}
}

// --- D10.2: no-leak en 500s -------------------------------------------------

func TestNoLeak_UserSearch_DBFailureKeepsContractMessage(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	svc := NewUserService(daos.NewUserDao(failing), nil, nil, nil, nil)

	_, err := svc.Search(nil, "alguenausuario")

	require.EqualError(t, err, "error al buscar usuarios")
	assertNoDriverLeak(t, err)
}

func TestNoLeak_TeamSearch_DBFailureKeepsContractMessage(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	svc := NewTeamService(
		daos.NewTeamDao(failing), daos.NewUserDao(db), nil, nil, daos.NewTeamUserDao(db),
		nil, nil, nil, nil, nil,
	)

	_, err := svc.Search(nil, 7, team.SearchFilters{}, 1)

	require.EqualError(t, err, "error al buscar equipos")
	assertNoDriverLeak(t, err)
}

func TestNoLeak_CalendarGetRange_DBFailureKeepsContractMessage(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "injnl01")
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(failing), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, nil, nil, nil, failing,
	)

	_, err := svc.GetRange(nil, group.ID, owner.ID, time.Now(), time.Now())

	require.EqualError(t, err, "error al listar calendario")
	assertNoDriverLeak(t, err)
}

// buildHistoryResponse re-envuelve los errores crudos del DAO bajo el prefijo
// genérico "error interno: %w" (fix del hallazgo del audit de task 6: el mapper
// del controller responde 500 con mensaje fijo, y este service ya no propaga
// el crudo). Este pin fija ese contrato: el mensaje conserva el prefijo
// genérico y el prefijo descriptivo del DAO, sin arrastrar texto del driver.
func TestNoLeak_WorkoutFeedbackHistory_DBFailureHasDescriptiveDAOError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	svc := NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(failing))

	_, err := svc.AthleteHistory(nil, 7, 7, workoutfeedback.HistoryQuery{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error interno: ")
	assert.Contains(t, err.Error(), "error searching workout feedback history")
	assertNoDriverLeak(t, err)
}
