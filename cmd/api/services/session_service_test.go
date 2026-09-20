package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/testutils"
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
	findBySessionFn     func(ctx *gin.Context, sessionID int64) ([]dbs.SessionExercise, error)
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
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Completa", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Completa", resp.Name)
}

func TestSessionService_Create_MissingRole(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "Incompleta", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"},
	}})

	assert.ErrorIs(t, err, ErrSessionMissingRole)
}

func TestSessionService_Create_InvalidRole(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "flying"},
	}})

	assert.ErrorIs(t, err, ErrSessionInvalidRole)
}

func TestSessionService_Create_ExerciseNotFound(t *testing.T) {
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) { return nil, nil }}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 7, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
}

func TestSessionService_Create_OwnerMismatch(t *testing.T) {
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Create(nil, 7, session.SessionRequest{OwnerID: 99, Name: "X", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Update_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Viejo"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.Equal(t, "Nuevo", resp.Name)
}

func TestSessionService_Update_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Update_Forbidden(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) {
		return &dbs.Session{ID: id, OwnerID: 99}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Nuevo", Exercises: validSessionExercises()})

	assert.ErrorIs(t, err, ErrCatalogForbidden)
}

func TestSessionService_Delete_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7}
	softDeleted := false
	sessionDao := &mockSessionDao{
		findByIDFn:   func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil },
		softDeleteFn: func(ctx *gin.Context, id int64) error { softDeleted = true; return nil },
	}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	err := svc.Delete(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, softDeleted)
}

func TestSessionService_Delete_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	err := svc.Delete(nil, 1, 7)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_Get_Success(t *testing.T) {
	existing := &dbs.Session{ID: 1, OwnerID: 7, Name: "Sesión"}
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return existing, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Get(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, "Sesión", resp.Name)
}

func TestSessionService_Get_NotFound(t *testing.T) {
	sessionDao := &mockSessionDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	_, err := svc.Get(nil, 1)

	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionService_List_Success(t *testing.T) {
	sessions := []dbs.Session{{ID: 1, OwnerID: 7, Name: "A"}, {ID: 2, OwnerID: 7, Name: "B"}}
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return sessions, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].Name)
	assert.Equal(t, "B", resp[1].Name)
}

func TestSessionService_List_Empty(t *testing.T) {
	sessionDao := &mockSessionDao{findByOwnerFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) { return nil, nil }}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.List(nil, 7)

	require.NoError(t, err)
	assert.Empty(t, resp)
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
	svc := NewSessionService(sessionDao, sessionExerciseDao, &mockExerciseDao{}, &mockGroupCalendarDao{}, nil)

	resp, err := svc.Clone(nil, 1, 7)

	require.NoError(t, err)
	assert.Equal(t, "Original (copia)", resp.Name)
	assert.True(t, replaced)
}

// TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints prueba la
// rama de divergencia de Update contra Postgres real: no es mockeable de forma
// significativa sin perder la garantía de atomicidad que el test quiere probar
// (la divergencia corre dentro de s.db.Transaction, con DAOs frescas sobre tx).
func TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
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
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: excludedGroup.ID, Date: date, Kind: "training", SessionInstanceID: &original.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: keptGroup.ID, Date: date, Kind: "training", SessionInstanceID: &original.ID}))

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
	require.NotNil(t, excludedDay.SessionInstanceID)
	assert.NotEqual(t, original.ID, *excludedDay.SessionInstanceID, "el grupo excluido debe apuntar al clon, no a la sesión original")

	clonedSessionID := *excludedDay.SessionInstanceID
	clonedExercises, err := sessionExerciseDao.FindBySession(nil, clonedSessionID)
	require.NoError(t, err)
	assert.Len(t, clonedExercises, 3, "el clon debe tener copia profunda de los 3 ejercicios")

	keptDay, err := calendarDao.FindByGroupAndDate(nil, keptGroup.ID, date)
	require.NoError(t, err)
	require.NotNil(t, keptDay.SessionInstanceID)
	assert.Equal(t, original.ID, *keptDay.SessionInstanceID, "el grupo no excluido debe seguir apuntando a la sesión original")

	updatedOriginal, err := sessionDao.FindByID(nil, original.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updatedOriginal.Name)
}

// TestSessionService_Update_AutoLocksPastDayWithoutExclusion prueba contra
// Postgres real: el día ya pasado dispara la rama de divergencia (igual que
// exclude_group_ids) aunque el llamado no incluya ningún grupo excluido —
// no es mockeable de forma significativa porque, una vez dentro de
// s.db.Transaction, Update usa DAOs frescas atadas a la tx (mismo motivo que
// TestSessionService_Update_WithExcludeGroupIDs_ClonesAndRepoints).
func TestSessionService_Update_AutoLocksPastDayWithoutExclusion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-autolock-past-owner@test.com", DNI: "50000092", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
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

	team := &dbs.Team{Name: "Equipo autolock past", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo autolock past", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	pastDate := time.Now().AddDate(0, 0, -3).Truncate(24 * time.Hour)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: pastDate, Kind: "training", IsPresencial: false, SessionInstanceID: &original.ID}))

	_, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: "Editada",
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)
	pastDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, pastDate)
	require.NoError(t, err)
	require.NotNil(t, pastDay.SessionInstanceID)
	assert.NotEqual(t, original.ID, *pastDay.SessionInstanceID, "el día pasado debe auto-clonarse aunque no venga en exclude_group_ids")
}

func TestSessionService_Update_FutureDayStaysLiveWithoutExclusion(t *testing.T) {
	futureDate := time.Now().AddDate(0, 0, 5)
	repointCalled := false
	calDao := &mockGroupCalendarDao{
		findBySessionIDFn: func(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error) {
			return []dbs.GroupCalendarDay{{ID: 502, GroupID: 1, Date: futureDate, Kind: "training", IsPresencial: false}}, nil
		},
		repointDaysByIDFn: func(ctx *gin.Context, dayIDs []int64, newSessionID int64) error {
			repointCalled = true
			return nil
		},
	}
	sessionDao := &mockSessionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return &dbs.Session{ID: id, OwnerID: 7}, nil },
	}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao, calDao, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Editada", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.False(t, repointCalled, "un día futuro no debe clonarse ni entrar a la transacción")
}

func TestSessionService_Update_TodayPresencialBeforeStartTimeStaysLive(t *testing.T) {
	// El horario de inicio debe ser estrictamente posterior a ahora dentro del
	// día actual. `now+2h` cruza la medianoche de noche y quedaría ANTES en el
	// reloj — para no depender de la hora de corrida, en ese caso se usa el
	// cierre del día (23:59) como horario de la presencial.
	now := time.Now()
	startClock := now.Add(2 * time.Hour)
	if startClock.Hour()*60+startClock.Minute() <= now.Hour()*60+now.Minute() {
		startClock = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
	}
	presencialTime := time.Date(0, 1, 1, startClock.Hour(), startClock.Minute(), 0, 0, time.UTC)
	repointCalled := false
	calDao := &mockGroupCalendarDao{
		findBySessionIDFn: func(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error) {
			return []dbs.GroupCalendarDay{{ID: 503, GroupID: 1, Date: time.Now(), Kind: "training", IsPresencial: true, PresencialTimeFrom: &presencialTime}}, nil
		},
		repointDaysByIDFn: func(ctx *gin.Context, dayIDs []int64, newSessionID int64) error {
			repointCalled = true
			return nil
		},
	}
	sessionDao := &mockSessionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Session, error) { return &dbs.Session{ID: id, OwnerID: 7}, nil },
	}
	exerciseDao := &mockExerciseDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
		return &dbs.Exercise{ID: id}, nil
	}}
	svc := NewSessionService(sessionDao, &mockSessionExerciseDao{}, exerciseDao, calDao, nil)

	_, err := svc.Update(nil, 1, 7, session.SessionRequest{OwnerID: 7, Name: "Editada", Exercises: validSessionExercises()})

	require.NoError(t, err)
	assert.False(t, repointCalled, "presencial de hoy antes de su horario sigue en vivo")
}

// TestSessionService_Update_TodayPresencialAfterStartTimeLocks — mismo motivo
// que TestSessionService_Update_AutoLocksPastDayWithoutExclusion para usar
// Postgres real en vez de mocks: la rama de auto-lock corre dentro de
// s.db.Transaction con DAOs frescas atadas a la tx.
func TestSessionService_Update_TodayPresencialAfterStartTimeLocks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-autolock-presencial-owner@test.com", DNI: "50000093", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
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

	team := &dbs.Team{Name: "Equipo autolock presencial", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo autolock presencial", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	// El horario de inicio debe estar estrictamente ANTES de ahora dentro del
	// día actual. `now-2h` cruza la medianoche de madrugada y quedaría DESPUÉS
	// en el reloj; en ese caso se usa el inicio del día (00:00) como horario
	// de la presencial para no depender de la hora de corrida.
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	past := now.Add(-2 * time.Hour)
	if past.Hour()*60+past.Minute() >= now.Hour()*60+now.Minute() {
		past = today
	}
	presencialTime := time.Date(0, 1, 1, past.Hour(), past.Minute(), 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: &presencialTime, SessionInstanceID: &original.ID,
	}))

	_, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: "Editada",
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)
	day, err := calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	require.NotNil(t, day.SessionInstanceID)
	assert.NotEqual(t, original.ID, *day.SessionInstanceID, "presencial de hoy después de su horario debe congelarse")
}

// TestSessionService_Update_TodayAsyncAlwaysLocks — mismo motivo que los dos
// tests anteriores para usar Postgres real.
func TestSessionService_Update_TodayAsyncAlwaysLocks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-autolock-async-owner@test.com", DNI: "50000094", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
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

	team := &dbs.Team{Name: "Equipo autolock async", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo autolock async", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: today, Kind: "training", IsPresencial: false, SessionInstanceID: &original.ID,
	}))

	_, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: "Editada",
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)
	day, err := calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	require.NotNil(t, day.SessionInstanceID)
	assert.NotEqual(t, original.ID, *day.SessionInstanceID, "asíncrono de hoy se congela aunque el día no haya terminado")
}

func TestSessionService_Update_AutoLocksPastDay_RealDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao, calendarDao, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "session-autolock-owner@test.com", DNI: "50000091", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
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

	team := &dbs.Team{Name: "Equipo autolock", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo autolock", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	pastDate := time.Now().AddDate(0, 0, -5).Truncate(24 * time.Hour)
	futureDate := time.Now().AddDate(0, 0, 5).Truncate(24 * time.Hour)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: pastDate, Kind: "training", SessionInstanceID: &original.ID}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: futureDate, Kind: "training", SessionInstanceID: &original.ID}))

	newName := "Sesión editada sin exclusión manual"
	_, err := svc.Update(nil, original.ID, owner.ID, session.SessionRequest{
		OwnerID: owner.ID, Name: newName,
		Exercises: []session.SessionExerciseRequest{
			{ExerciseID: warmup.ID, Role: "warmup"},
			{ExerciseID: main.ID, Role: "main"},
			{ExerciseID: cooldown.ID, Role: "cooldown"},
		},
	})

	require.NoError(t, err)

	pastDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, pastDate)
	require.NoError(t, err)
	require.NotNil(t, pastDay.SessionInstanceID)
	assert.NotEqual(t, original.ID, *pastDay.SessionInstanceID, "el día pasado debe apuntar a un clon, no a la sesión original ya editada")

	futureDay, err := calendarDao.FindByGroupAndDate(nil, group.ID, futureDate)
	require.NoError(t, err)
	require.NotNil(t, futureDay.SessionInstanceID)
	assert.Equal(t, original.ID, *futureDay.SessionInstanceID, "el día futuro debe seguir apuntando a la sesión original ya editada")

	updatedOriginal, err := sessionDao.FindByID(nil, original.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updatedOriginal.Name)
}
