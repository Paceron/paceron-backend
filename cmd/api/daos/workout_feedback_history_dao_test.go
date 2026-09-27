package daos

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// historyUser crea un usuario con nombre custom (los del fixture de historial
// ordenan el pool de atletas por name).
func historyUser(db *gorm.DB, email, dni, name string) *dbs.User {
	user := &dbs.User{
		Name:      name,
		Surname:   "Runner",
		Email:     email,
		DNI:       dni,
		BirthDate: time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "hashed",
	}
	db.Create(user)
	return user
}

// historyFeedback crea un feedback válido directo por GORM (bypass del DAO),
// con fecha configurable para los tests de rango/orden.
func historyFeedback(t *testing.T, db *gorm.DB, athleteID, sessionID, exerciseID int64, teamID *int64, setNumber int, date time.Time) *dbs.WorkoutFeedback {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	fb := &dbs.WorkoutFeedback{
		TeamID:              teamID,
		AssignedSessionID:   sessionID,
		AssignedExerciseID:  exerciseID,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: athleteID,
		ReportSource:        "corredor",
		SessionDate:         date,
		SetNumber:           setNumber,
		MediaURLs:           media,
	}
	require.NoError(t, db.Create(fb).Error)
	return fb
}

type workoutFeedbackHistoryFixture struct {
	teamA, teamB   *dbs.Team
	groupA, groupB *dbs.Group
	athleteA       *dbs.User // "Anita"
	athleteB       *dbs.User // "Benja"
	sessionInst    *dbs.SessionInstance
	exA            *dbs.ExerciseInstance // "Zancada larga", con source_exercise_id
	sourceA        int64                 // id de catálogo de exA (4242)
	exB            *dbs.ExerciseInstance // "Fartlek corto", sin source
	fbNormal       *dbs.WorkoutFeedback  // teamA, día groupA, exA, set 0, Jan 15
	fbNormalSet1   *dbs.WorkoutFeedback  // teamA, día groupA, exA, set 1, Jan 15
	fbTie1         *dbs.WorkoutFeedback  // teamA, día groupA, exA, set 2, Jan 18
	fbTie2         *dbs.WorkoutFeedback  // teamA, día groupA, exB, set 2, Jan 18 (desempate con fbTie1)
	fbOrphan       *dbs.WorkoutFeedback  // teamA, SIN día, exB, set 0, Jan 20 → group null
	fbNoName       *dbs.WorkoutFeedback  // teamA, día groupA, instancia inexistente, set 0, Jan 10 → exercise_name null
	fbNoTeam       *dbs.WorkoutFeedback  // sin team, día groupA, exA, set 0, Jan 22
	fbOtherTeam    *dbs.WorkoutFeedback  // teamB, sin día, exB, set 0, Jan 25
	fbDeleted      *dbs.WorkoutFeedback  // teamA, día groupA, exA, set 9, Jan 15 → soft-deleteado
}

// setupWorkoutFeedbackHistoryFixture arma el escenario completo del design D8:
// 2 equipos/grupos, un día de calendario con instancia, y feedbacks normal,
// huérfano sin día, sin team, de otro equipo y soft-deleteado.
func setupWorkoutFeedbackHistoryFixture(t *testing.T, db *gorm.DB) *workoutFeedbackHistoryFixture {
	t.Helper()
	dao := NewWorkoutFeedbackDao(db)

	ownerA := persistUser(db, "wf-history-owner-a@test.com", "91000001")
	ownerB := persistUser(db, "wf-history-owner-b@test.com", "91000002")
	teamA := testTeam(db, "wf_history_team_a", ownerA.ID)
	teamB := testTeam(db, "wf_history_team_b", ownerB.ID)
	groupA := testGroup(db, "wf_history_grupo_a", teamA.ID)
	groupB := testGroup(db, "wf_history_grupo_b", teamB.ID)

	athleteA := historyUser(db, "wf-history-atleta-a@test.com", "91000003", "Anita")
	athleteB := historyUser(db, "wf-history-atleta-b@test.com", "91000004", "Benja")

	sessionInst := &dbs.SessionInstance{Name: "WF Hist Sesión"}
	require.NoError(t, db.Create(sessionInst).Error)

	sourceA := int64(4242)
	exA := &dbs.ExerciseInstance{Name: "Zancada larga", Kind: "running", SourceExerciseID: &sourceA}
	exB := &dbs.ExerciseInstance{Name: "Fartlek corto", Kind: "running"}
	require.NoError(t, db.Create(exA).Error)
	require.NoError(t, db.Create(exB).Error)

	day := &dbs.GroupCalendarDay{
		GroupID:           groupA.ID,
		Date:              time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Kind:              "training",
		SessionInstanceID: &sessionInst.ID,
	}
	require.NoError(t, db.Create(day).Error)

	jan := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	f := &workoutFeedbackHistoryFixture{
		teamA: teamA, teamB: teamB,
		groupA: groupA, groupB: groupB,
		athleteA: athleteA, athleteB: athleteB,
		sourceA:     sourceA,
		sessionInst: sessionInst,
		exA:         exA, exB: exB,
	}
	f.fbNormal = historyFeedback(t, db, athleteA.ID, sessionInst.ID, exA.ID, &teamA.ID, 0, jan(15))
	f.fbNormalSet1 = historyFeedback(t, db, athleteA.ID, sessionInst.ID, exA.ID, &teamA.ID, 1, jan(15))
	f.fbTie1 = historyFeedback(t, db, athleteA.ID, sessionInst.ID, exA.ID, &teamA.ID, 2, jan(18))
	f.fbTie2 = historyFeedback(t, db, athleteB.ID, sessionInst.ID, exB.ID, &teamA.ID, 2, jan(18))
	f.fbOrphan = historyFeedback(t, db, athleteA.ID, 987654321, exB.ID, &teamA.ID, 0, jan(20))
	f.fbNoName = historyFeedback(t, db, athleteA.ID, sessionInst.ID, 777777, &teamA.ID, 0, jan(10))
	f.fbNoTeam = historyFeedback(t, db, athleteB.ID, sessionInst.ID, exA.ID, nil, 0, jan(22))
	f.fbOtherTeam = historyFeedback(t, db, athleteB.ID, 555000111, exB.ID, &teamB.ID, 0, jan(25))
	f.fbDeleted = historyFeedback(t, db, athleteA.ID, sessionInst.ID, exA.ID, &teamA.ID, 9, jan(15))
	require.NoError(t, dao.SoftDelete(nil, f.fbDeleted.ID))
	return f
}

func ptrTime(t time.Time) *time.Time { return &t }

func historyRowByID(rows []dbs.WorkoutFeedbackHistoryRow, id int64) *dbs.WorkoutFeedbackHistoryRow {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func historyRowIDs(rows []dbs.WorkoutFeedbackHistoryRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestWorkoutFeedbackDao_HistorySearch_TeamScopeEnrichedRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)

	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &f.teamA.ID}, "feedback_date", "desc", 0, 0)

	require.NoError(t, err)
	require.Len(t, rows, 6) // todos los de teamA activos: 7 menos el soft-deleteado
	assert.NotContains(t, historyRowIDs(rows), f.fbDeleted.ID)
	assert.NotContains(t, historyRowIDs(rows), f.fbOtherTeam.ID)

	// Fila normal: joins completos.
	row := historyRowByID(rows, f.fbNormal.ID)
	require.NotNil(t, row)
	assert.Equal(t, f.athleteA.ID, row.AthleteUserID)
	require.NotNil(t, row.TeamID)
	assert.Equal(t, f.teamA.ID, *row.TeamID)
	require.NotNil(t, row.GroupID)
	assert.Equal(t, f.groupA.ID, *row.GroupID)
	assert.Equal(t, f.fbNormal.SessionDate, row.SessionDate)
	assert.Equal(t, 0, row.SetNumber)
	assert.Equal(t, f.exA.ID, row.ExerciseID)
	require.NotNil(t, row.ExerciseName)
	assert.Equal(t, "Zancada larga", *row.ExerciseName)
	require.NotNil(t, row.CatalogExerciseID)
	assert.Equal(t, int64(4242), *row.CatalogExerciseID)
	require.NotNil(t, row.SessionName)
	assert.Equal(t, "WF Hist Sesión", *row.SessionName)
	assert.Equal(t, f.sessionInst.ID, row.SessionInstanceID)

	// Huérfano sin día: group/session nulls, pero team y ejercicio sí.
	orphan := historyRowByID(rows, f.fbOrphan.ID)
	require.NotNil(t, orphan)
	assert.Nil(t, orphan.GroupID)
	assert.Nil(t, orphan.SessionName)
	assert.Equal(t, int64(987654321), orphan.SessionInstanceID)
	require.NotNil(t, orphan.TeamID)
	assert.Equal(t, f.teamA.ID, *orphan.TeamID)
	require.NotNil(t, orphan.ExerciseName)
	assert.Equal(t, "Fartlek corto", *orphan.ExerciseName)
	assert.Nil(t, orphan.CatalogExerciseID)

	// Instancia inexistente: exercise_name null.
	noName := historyRowByID(rows, f.fbNoName.ID)
	require.NotNil(t, noName)
	assert.Nil(t, noName.ExerciseName)
	assert.Nil(t, noName.CatalogExerciseID)
	require.NotNil(t, noName.SessionName)
}

func TestWorkoutFeedbackDao_HistorySearch_AthleteScopeIncludesNoTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)

	rowsA, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{AthleteUserID: &f.athleteA.ID}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	require.Len(t, rowsA, 5) // normal, set1, tie1, orphan, noName (deleted excluido)
	assert.ElementsMatch(t,
		[]int64{f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID, f.fbOrphan.ID, f.fbNoName.ID},
		historyRowIDs(rowsA))

	rowsB, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{AthleteUserID: &f.athleteB.ID}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	require.Len(t, rowsB, 3) // tie2, noTeam, otherTeam

	// Sin team: team null pero group presente (su sesión está asignada a un día).
	noTeam := historyRowByID(rowsB, f.fbNoTeam.ID)
	require.NotNil(t, noTeam)
	assert.Nil(t, noTeam.TeamID)
	require.NotNil(t, noTeam.GroupID)
	assert.Equal(t, f.groupA.ID, *noTeam.GroupID)
}

func TestWorkoutFeedbackDao_HistorySearch_GroupFilter(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)

	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{GroupID: &f.groupA.ID}, "feedback_date", "desc", 0, 0)

	require.NoError(t, err)
	require.Len(t, rows, 6) // los del día de groupA, con o sin team; huérfano fuera
	assert.ElementsMatch(t,
		[]int64{f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID, f.fbTie2.ID, f.fbNoName.ID, f.fbNoTeam.ID},
		historyRowIDs(rows))

	rowsB, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{GroupID: &f.groupB.ID}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.Empty(t, rowsB)
}

func TestWorkoutFeedbackDao_HistorySearch_DateRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	jan := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

	teamA := f.teamA.ID
	// Un solo día (from == to): incluye los feedbacks de ese día, no el soft-deleteado.
	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, DateFrom: ptrTime(jan(15)), DateTo: ptrTime(jan(15))}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{f.fbNormal.ID, f.fbNormalSet1.ID}, historyRowIDs(rows))

	// Rango abierto: cruza equipos si no hay scope de team.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{DateFrom: ptrTime(jan(16)), DateTo: ptrTime(jan(22))}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]int64{f.fbTie1.ID, f.fbTie2.ID, f.fbOrphan.ID, f.fbNoTeam.ID},
		historyRowIDs(rows))

	// Rango sin resultados.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, DateFrom: ptrTime(jan(1)), DateTo: ptrTime(jan(5))}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestWorkoutFeedbackDao_HistorySearch_SecondLevelFilters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID

	byExercise, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, ExerciseInstanceID: &f.sourceA}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]int64{f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID},
		historyRowIDs(byExercise))

	bySet, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, SetNumber: &[]int{2}[0]}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{f.fbTie1.ID, f.fbTie2.ID}, historyRowIDs(bySet))

	byAthlete, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, AthleteFilterUserID: &f.athleteA.ID}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]int64{f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID, f.fbOrphan.ID, f.fbNoName.ID},
		historyRowIDs(byAthlete))

	set0 := 0
	combined, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, ExerciseInstanceID: &f.sourceA, SetNumber: &set0}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.fbNormal.ID}, historyRowIDs(combined))
}

func TestWorkoutFeedbackDao_HistorySearch_SortWhitelistAndTiebreak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID

	// feedback_date desc: empates Jan 15 y Jan 18 resueltos por id DESC.
	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{
		f.fbOrphan.ID,     // Jan 20
		f.fbTie2.ID,       // Jan 18, id mayor primero
		f.fbTie1.ID,       // Jan 18
		f.fbNormalSet1.ID, // Jan 15, id mayor primero
		f.fbNormal.ID,     // Jan 15
		f.fbNoName.ID,     // Jan 10
	}, historyRowIDs(rows))

	// asc: orden exactamente inverso (incluido el desempate por id).
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{
		f.fbNoName.ID, f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID, f.fbTie2.ID, f.fbOrphan.ID,
	}, historyRowIDs(rows))

	// set_number desc: empates de set también por id DESC.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "set_number", "desc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{
		f.fbTie2.ID, f.fbTie1.ID, // set 2
		f.fbNormalSet1.ID,                           // set 1
		f.fbNoName.ID, f.fbOrphan.ID, f.fbNormal.ID, // set 0, id DESC
	}, historyRowIDs(rows))

	// set_number asc.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "set_number", "asc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{
		f.fbNormal.ID, f.fbOrphan.ID, f.fbNoName.ID,
		f.fbNormalSet1.ID,
		f.fbTie1.ID, f.fbTie2.ID,
	}, historyRowIDs(rows))

	// exercise_name asc: "Fartlek corto" < "Zancada larga"; la fila sin nombre
	// no se posiciona acá (orden de NULLs lo define Postgres).
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "exercise_name", "asc", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 6)
	assert.Equal(t, []int64{f.fbTie2.ID, f.fbOrphan.ID, f.fbNormal.ID, f.fbNormalSet1.ID, f.fbTie1.ID},
		historyRowIDs(rows)[:5])

	// exercise_name desc (Postgres ordena NULLs primero en DESC, la fila sin
	// nombre no se posiciona acá).
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "exercise_name", "desc", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 6)
	assert.Equal(t, []int64{f.fbTie1.ID, f.fbNormalSet1.ID, f.fbNormal.ID, f.fbOrphan.ID, f.fbTie2.ID},
		historyRowIDs(rows)[1:])

	// sort fuera de whitelist → default feedback_date desc (el service valida 400 antes).
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "bogus", "desc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{
		f.fbOrphan.ID, f.fbTie2.ID, f.fbTie1.ID, f.fbNormalSet1.ID, f.fbNormal.ID, f.fbNoName.ID,
	}, historyRowIDs(rows))
}

func TestWorkoutFeedbackDao_HistorySearch_Pagination(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID

	full, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 0, 0)
	require.NoError(t, err)
	require.Len(t, full, 6)

	page1, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, full[:2], page1)

	page2, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, full[2:4], page2)

	last, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 2, 4)
	require.NoError(t, err)
	assert.Equal(t, full[4:6], last)

	beyond, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA}, "feedback_date", "asc", 2, 100)
	require.NoError(t, err)
	assert.Empty(t, beyond)
}

func TestWorkoutFeedbackDao_HistoryCount_MatchesFilters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID
	jan := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

	total, err := dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA})
	require.NoError(t, err)
	assert.Equal(t, int64(6), total) // sin paginación; soft-deleteado fuera

	total, err = dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, SetNumber: &[]int{9}[0]})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total) // solo existía el soft-deleteado con set 9

	total, err = dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, ExerciseInstanceID: &f.sourceA})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	total, err = dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{GroupID: &f.groupA.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)

	total, err = dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, DateFrom: ptrTime(jan(16)), DateTo: ptrTime(jan(22))})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	total, err = dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{AthleteUserID: &f.athleteB.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
}

func TestWorkoutFeedbackDao_HistoryAvailableAthletes_FirstLevelOnly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID
	jan := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

	// Pool completo del team, ordenado por name ("Anita" < "Benja").
	athletes, err := dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{
		{ID: f.athleteA.ID, Name: "Anita"},
		{ID: f.athleteB.ID, Name: "Benja"},
	}, athletes)

	// Segundo nivel seteado: ignorado, mismo pool.
	athletes, err = dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, ExerciseInstanceID: &f.sourceA})
	require.NoError(t, err)
	assert.Len(t, athletes, 2)
	athletes, err = dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, SetNumber: &[]int{2}[0], AthleteFilterUserID: &f.athleteA.ID})
	require.NoError(t, err)
	assert.Len(t, athletes, 2)

	// Primer nivel sí aplica: un solo día deja solo al atleta de ese día.
	athletes, err = dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, DateFrom: ptrTime(jan(15)), DateTo: ptrTime(jan(15))})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{{ID: f.athleteA.ID, Name: "Anita"}}, athletes)

	// Scope por atleta (corredor): el pool es uno mismo.
	athletes, err = dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{AthleteUserID: &f.athleteA.ID})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{{ID: f.athleteA.ID, Name: "Anita"}}, athletes)

	// Filtro por grupo: atletas con feedbacks en el día de groupA.
	athletes, err = dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{GroupID: &f.groupA.ID})
	require.NoError(t, err)
	assert.Len(t, athletes, 2)
}

func TestWorkoutFeedbackDao_HistoryAvailableExercises_FirstLevelOnly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)
	teamA := f.teamA.ID
	jan := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

	// Ordenado por name ("Fartlek corto" < "Zancada larga"); el pool id de
	// exA es su ejercicio de catálogo (4242) y el de exB su propia instancia
	// (legado sin origen). La instancia inexistente (777777) no entra (INNER JOIN).
	exercises, err := dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{
		{ID: f.exB.ID, Name: "Fartlek corto"},
		{ID: 4242, Name: "Zancada larga"},
	}, exercises)

	// Segundo nivel seteado: ignorado.
	exercises, err = dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, SetNumber: &[]int{1}[0], AthleteFilterUserID: &f.athleteB.ID})
	require.NoError(t, err)
	assert.Len(t, exercises, 2)

	// Primer nivel sí aplica: un solo día deja solo el ejercicio de ese día.
	exercises, err = dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{TeamID: &teamA, DateFrom: ptrTime(jan(15)), DateTo: ptrTime(jan(15))})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{{ID: 4242, Name: "Zancada larga"}}, exercises)

	// Scope por atleta sin team: ejercicios de sus feedbacks activos.
	exercises, err = dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{AthleteUserID: &f.athleteA.ID})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{
		{ID: f.exB.ID, Name: "Fartlek corto"},
		{ID: 4242, Name: "Zancada larga"},
	}, exercises)

	// Filtro por grupo con el pool de ejercicios (join gcd también acá):
	// los feedbacks del día de groupA cubren exA y exB.
	exercises, err = dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{GroupID: &f.groupA.ID})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{
		{ID: f.exB.ID, Name: "Fartlek corto"},
		{ID: 4242, Name: "Zancada larga"},
	}, exercises)
}

func TestWorkoutFeedbackDao_History_NoScopeReturnsAllActive(t *testing.T) {
	// El service siempre setea scope (atleta o team); el DAO con filtros vacíos
	// no agrega ninguna restricción implícita — este test fija ese contrato.
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	f := setupWorkoutFeedbackHistoryFixture(t, db)

	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{}, "feedback_date", "desc", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 8) // 9 fixture menos el soft-deleteado
	assert.NotContains(t, historyRowIDs(rows), f.fbDeleted.ID)

	total, err := dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{})
	require.NoError(t, err)
	assert.Equal(t, int64(8), total)

	athletes, err := dao.HistoryAvailableAthletes(nil, WorkoutFeedbackHistoryFilters{})
	require.NoError(t, err)
	assert.Len(t, athletes, 2)

	exercises, err := dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{})
	require.NoError(t, err)
	assert.Len(t, exercises, 2)
}

func TestWorkoutFeedbackDao_HistorySearch_ExerciseFilterFamiliaCatalogo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	// Semántica del filtro de ejercicio (ajuste post-feedback frontend):
	// matchea por familia de catálogo, no por instancia puntual.
	owner := persistUser(db, "wf-familia-owner@test.com", "92000001")
	team := testTeam(db, "wf_familia_team", owner.ID)
	athlete := historyUser(db, "wf-familia-atleta@test.com", "92000002", "Caro")

	source := int64(5555)
	instA := &dbs.ExerciseInstance{Name: "Trote", Kind: "running", SourceExerciseID: &source}
	instB := &dbs.ExerciseInstance{Name: "Trote", Kind: "running", SourceExerciseID: &source}
	legacy := &dbs.ExerciseInstance{Name: "Zancada legado", Kind: "running"}
	require.NoError(t, db.Create(instA).Error)
	require.NoError(t, db.Create(instB).Error)
	require.NoError(t, db.Create(legacy).Error)

	fbSession := int64(1)
	fbA := historyFeedback(t, db, athlete.ID, fbSession, instA.ID, &team.ID, 0, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	fbB := historyFeedback(t, db, athlete.ID, fbSession, instB.ID, &team.ID, 0, time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC))
	fbLegacy := historyFeedback(t, db, athlete.ID, fbSession, legacy.ID, &team.ID, 0, time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC))

	// Filtro por el id de catálogo: trae las filas de TODAS las instancias
	// de esa familia (instA + instB), no solo una puntual.
	rows, err := dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &team.ID, ExerciseInstanceID: &source}, "feedback_date", "asc", 0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{fbA.ID, fbB.ID}, historyRowIDs(rows))

	total, err := dao.HistoryCount(nil, WorkoutFeedbackHistoryFilters{TeamID: &team.ID, ExerciseInstanceID: &source})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// Instancia legado sin origen: su pool id es el propio id de instancia
	// y el filtro matchea por ese id.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &team.ID, ExerciseInstanceID: &legacy.ID}, "feedback_date", "asc", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{fbLegacy.ID}, historyRowIDs(rows))

	// Un id de instancia CON origen ya no matchea como instancia: solo
	// colisionaría si coincidiera con un id de catálogo.
	rows, err = dao.HistorySearch(nil, WorkoutFeedbackHistoryFilters{TeamID: &team.ID, ExerciseInstanceID: &instA.ID}, "feedback_date", "asc", 0, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Pool dedupeado por familia: instA+instB → un solo item con id de
	// catálogo y nombre de la instancia representativa (id menor).
	exercises, err := dao.HistoryAvailableExercises(nil, WorkoutFeedbackHistoryFilters{TeamID: &team.ID})
	require.NoError(t, err)
	assert.Equal(t, []dbs.IDName{
		{ID: source, Name: "Trote"},
		{ID: legacy.ID, Name: "Zancada legado"},
	}, exercises)
}
