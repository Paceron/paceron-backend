package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockAttendanceDao struct {
	createFn            func(ctx *gin.Context, attendance *dbs.Attendance) error
	searchFn            func(ctx *gin.Context, filters daos.AttendanceSearchFilters) ([]dbs.Attendance, error)
	teamExistsFn        func(ctx *gin.Context, teamID int64) (bool, error)
	isTeamOwnerFn       func(ctx *gin.Context, teamID, userID int64) (bool, error)
	userInTeamOwnedByFn func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
	getTeamUserRoleFn   func(ctx *gin.Context, teamID, userID int64) (string, error)
	findSessionCtxFn    func(ctx *gin.Context, sessionInstanceID int64) (*daos.AttendanceSessionContext, error)
	findPastSessionsFn  func(ctx *gin.Context, groupID, teamID int64) ([]daos.SessionAttendanceOption, error)
	findRosterFn        func(ctx *gin.Context, groupID, teamID, sessionInstanceID int64, sessionDate time.Time) ([]daos.SessionAttendanceRow, *daos.SessionAttendanceAggregates, error)
	bulkUpsertFn        func(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error)
	findByIDFn          func(ctx *gin.Context, attendanceID int64) (*dbs.Attendance, error)
	deleteByIDFn        func(ctx *gin.Context, attendanceID, teamID int64) (bool, error)
	// called cuenta las invocaciones por método, para asentar que el refactor de
	// teamRoleFor no agregó consultas a la DB.
	called map[string]int
}

func (m *mockAttendanceDao) Create(ctx *gin.Context, a *dbs.Attendance) error {
	if m.createFn != nil {
		return m.createFn(ctx, a)
	}
	return nil
}

func (m *mockAttendanceDao) Search(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, f)
	}
	return nil, nil
}

func (m *mockAttendanceDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	if m.teamExistsFn != nil {
		return m.teamExistsFn(ctx, teamID)
	}
	return false, nil
}

func (m *mockAttendanceDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	m.record("IsTeamOwner")
	if m.isTeamOwnerFn != nil {
		return m.isTeamOwnerFn(ctx, teamID, userID)
	}
	return false, nil
}

func (m *mockAttendanceDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	if m.userInTeamOwnedByFn != nil {
		return m.userInTeamOwnedByFn(ctx, targetUserID, ownerUserID)
	}
	return false, nil
}

func (m *mockAttendanceDao) GetTeamUserRole(ctx *gin.Context, teamID, userID int64) (string, error) {
	m.record("GetTeamUserRole")
	if m.getTeamUserRoleFn != nil {
		return m.getTeamUserRoleFn(ctx, teamID, userID)
	}
	return "", nil
}

func (m *mockAttendanceDao) FindSessionContext(ctx *gin.Context, sessionInstanceID int64) (*daos.AttendanceSessionContext, error) {
	m.record("FindSessionContext")
	if m.findSessionCtxFn != nil {
		return m.findSessionCtxFn(ctx, sessionInstanceID)
	}
	return nil, nil
}

func (m *mockAttendanceDao) record(method string) {
	if m.called == nil {
		m.called = map[string]int{}
	}
	m.called[method]++
}

func (m *mockAttendanceDao) FindPastPresencialSessionsForGroup(ctx *gin.Context, groupID, teamID int64) ([]daos.SessionAttendanceOption, error) {
	m.record("FindPastPresencialSessionsForGroup")
	if m.findPastSessionsFn != nil {
		return m.findPastSessionsFn(ctx, groupID, teamID)
	}
	return nil, nil
}

func (m *mockAttendanceDao) FindGroupRosterWithAttendance(ctx *gin.Context, groupID, teamID, sessionInstanceID int64, sessionDate time.Time) ([]daos.SessionAttendanceRow, *daos.SessionAttendanceAggregates, error) {
	m.record("FindGroupRosterWithAttendance")
	if m.findRosterFn != nil {
		return m.findRosterFn(ctx, groupID, teamID, sessionInstanceID, sessionDate)
	}
	return nil, nil, nil
}

// qrCoachDao es el contexto de sesión que espera GenerateQR para un equipo/sesión
// válidos: entrenador, equipo existente, sesión presencial no cancelada.
func qrCoachDao(coachID int64) *mockAttendanceDao {
	return &mockAttendanceDao{
		teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
		isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return userID == coachID, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
		findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			return &daos.AttendanceSessionContext{TeamID: 5, GroupID: 7, Kind: "training", IsPresencial: true}, nil
		},
	}
}

func TestAttendanceService_GenerateQR_DeterministicAndURL(t *testing.T) {
	svc := NewAttendanceService(qrCoachDao(1), nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	qr1, err := svc.GenerateQR(nil, 1, 5, 9)
	require.NoError(t, err)
	qr2, err := svc.GenerateQR(nil, 1, 5, 9)
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/attendance/register?team_id=5&session_instance_id=9", qr1.URLEncoded)
	assert.Equal(t, qr1.QRCodeBase64, qr2.QRCodeBase64)
	assert.NotEmpty(t, qr1.QRCodeBase64)
}

func TestAttendanceService_GenerateQR_BaseURLWithoutTrailingSlash(t *testing.T) {
	svc := NewAttendanceService(qrCoachDao(1), nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080/")

	// El equipo tiene que coincidir con el del contexto de sesión que devuelve
	// qrCoachDao (5), o la validación de equipo lo rechaza antes de generar el QR.
	qr, err := svc.GenerateQR(nil, 1, 5, 2)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/attendance/register?team_id=5&session_instance_id=2", qr.URLEncoded)
}

// TestAttendanceService_GenerateQR_AuthorizationAndSessionValidation cubre los 4
// escenarios de QR de la spec: emisión OK, 403 para un no-entrenador, 422 para una
// sesión inválida y 404 para un equipo inexistente.
func TestAttendanceService_GenerateQR_AuthorizationAndSessionValidation(t *testing.T) {
	t.Run("un usuario que no es entrenador del equipo recibe 403", func(t *testing.T) {
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return false, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.GenerateQR(nil, 77, 5, 9)

		require.ErrorIs(t, err, ErrForbiddenAttendance)
	})

	t.Run("sesion no presencial es 422", func(t *testing.T) {
		dao := qrCoachDao(1)
		dao.findSessionCtxFn = func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			return &daos.AttendanceSessionContext{TeamID: 5, GroupID: 7, Kind: "training", IsPresencial: false}, nil
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.GenerateQR(nil, 1, 5, 9)

		require.ErrorIs(t, err, ErrAttendanceSessionNotPresencial)
	})

	t.Run("sesion cancelada es 422", func(t *testing.T) {
		dao := qrCoachDao(1)
		dao.findSessionCtxFn = func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			// La cancelación se representa con el kind del día de calendario, no
			// con un campo aparte.
			return &daos.AttendanceSessionContext{TeamID: 5, GroupID: 7, Kind: string(constants.GroupCalendarDayKindCancelled), IsPresencial: true}, nil
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.GenerateQR(nil, 1, 5, 9)

		require.ErrorIs(t, err, ErrAttendanceSessionCancelled)
	})

	t.Run("equipo inexistente es 404", func(t *testing.T) {
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return false, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.GenerateQR(nil, 1, 5, 9)

		require.ErrorIs(t, err, ErrTeamNotFound)
	})

	t.Run("una sesion futura es legitima: no se filtra por fecha", func(t *testing.T) {
		// El caso NORMAL de uso es emitir el QR antes de la clase. Si se filtrara
		// por fecha pasada, el entrenador no podria emitirlo a tiempo.
		dao := qrCoachDao(1)
		dao.findSessionCtxFn = func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			return &daos.AttendanceSessionContext{
				TeamID:       5,
				GroupID:      7,
				Kind:         "training",
				IsPresencial: true,
				Date:         time.Now().Add(72 * time.Hour),
			}, nil
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, nil, "http://x")

		qr, err := svc.GenerateQR(nil, 1, 5, 9)

		require.NoError(t, err)
		assert.NotEmpty(t, qr.QRCodeBase64)
	})
}

// registerMemberDao es el contexto de Register para el camino feliz: la sesión
// existe, pertenece al grupo 7 y el corredor es miembro activo.
func registerMemberDao(create func(ctx *gin.Context, a *dbs.Attendance) error) *mockAttendanceDao {
	return &mockAttendanceDao{
		findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			return &daos.AttendanceSessionContext{TeamID: 5, GroupID: 7, Kind: "training", IsPresencial: true}, nil
		},
		createFn: create,
	}
}

func TestAttendanceService_Register_Created(t *testing.T) {
	mock := registerMemberDao(func(ctx *gin.Context, a *dbs.Attendance) error {
		assert.Equal(t, int64(7), a.UserID)
		assert.Equal(t, int64(5), a.TeamID)
		assert.Equal(t, int64(9), a.TrainingSessionID)
		return nil
	})
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	created, sessionCtx, err := svc.Register(nil, 7, 5, 9)

	require.NoError(t, err)
	assert.True(t, created)
	// El contexto se devuelve para que el front pueda armar el deep link de la
	// sesión: sin grupo ni fecha no hay ruta a la que llevar al corredor.
	require.NotNil(t, sessionCtx)
	assert.Equal(t, int64(7), sessionCtx.GroupID, "el grupo del fixture: es lo que arma el deep link del front")
}

func TestAttendanceService_Register_AlreadyExists(t *testing.T) {
	// La idempotencia se preserva: 201 la primera vez, 200 la segunda, sin
	// insertar duplicado. El caso "ya existe" NO es un error.
	mock := registerMemberDao(func(ctx *gin.Context, a *dbs.Attendance) error {
		return daos.ErrAttendanceAlreadyExists
	})
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	created, sessionCtx, err := svc.Register(nil, 7, 5, 9)

	require.NoError(t, err)
	assert.False(t, created)
	// Idempotencia con contexto: el 200 también tiene que poder llevar al
	// corredor a la sesión, no solo el 201.
	require.NotNil(t, sessionCtx)
	assert.Equal(t, int64(7), sessionCtx.GroupID, "el grupo del fixture: es lo que arma el deep link del front")
}

func TestAttendanceService_Register_DAOError(t *testing.T) {
	mock := registerMemberDao(func(ctx *gin.Context, a *dbs.Attendance) error {
		return errors.New("db caída")
	})
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	created, sessionCtx, err := svc.Register(nil, 7, 5, 9)

	require.Error(t, err)
	assert.False(t, created)
	// Con error no hay contexto: el front no tiene a dónde llevar al corredor.
	assert.Nil(t, sessionCtx)
}

// TestAttendanceService_Register_RequiresGroupMembership cubre la brecha que este
// change cierra: antes, el handler solo verificaba que hubiera un usuario
// autenticado, así que cualquiera podía cargar asistencia en nombre de cualquiera,
// para cualquier equipo y sesión.
func TestAttendanceService_Register_RequiresGroupMembership(t *testing.T) {
	t.Run("un corredor que no es miembro del grupo recibe 403 y no inserta", func(t *testing.T) {
		inserted := false
		mock := registerMemberDao(func(ctx *gin.Context, a *dbs.Attendance) error {
			inserted = true
			return nil
		})
		gu := &attendanceGroupUserStub{
			isMemberFn: func(ctx *gin.Context, groupID, userID int64, sessionDate time.Time) (bool, error) { return false, nil },
		}
		svc := NewAttendanceService(mock, nil, gu, nil, "http://x")

		created, sessionCtx, err := svc.Register(nil, 77, 5, 9)

		require.ErrorIs(t, err, ErrAttendanceNotGroupMember)
		assert.False(t, created)
		assert.Nil(t, sessionCtx)
		assert.False(t, inserted, "sin membresía no se escribe ninguna fila")
	})

	t.Run("la membresía se consulta contra el grupo y la fecha de la sesión", func(t *testing.T) {
		sessionDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
		mock := &mockAttendanceDao{
			findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
				return &daos.AttendanceSessionContext{TeamID: 5, GroupID: 7, Kind: "training", IsPresencial: true, Date: sessionDate}, nil
			},
			createFn: func(ctx *gin.Context, a *dbs.Attendance) error { return nil },
		}
		gu := &attendanceGroupUserStub{}
		svc := NewAttendanceService(mock, nil, gu, nil, "http://x")

		_, _, err := svc.Register(nil, 12, 5, 9)

		require.NoError(t, err)
		assert.Equal(t, []int64{7}, gu.lastGroupIDs, "contra el grupo de la sesión, no contra el del request")
		assert.Equal(t, sessionDate, gu.lastDate, "D7: al grupo se pertenecía el día de la clase, no hoy")
	})

	t.Run("sesion sin grupo asignado es 403 y no 404", func(t *testing.T) {
		mock := &mockAttendanceDao{
			findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) { return nil, nil },
			createFn:         func(ctx *gin.Context, a *dbs.Attendance) error { return nil },
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://x")

		_, _, err := svc.Register(nil, 12, 5, 999)

		// 404 confirmaría que ese id de sesión existe; no hay grupo al que
		// pertenecer, así que la respuesta no filtra nada.
		require.ErrorIs(t, err, ErrAttendanceNotGroupMember)
	})
}

func TestAttendanceService_Search_TeamIDRequired(t *testing.T) {
	mock := &mockAttendanceDao{}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{})

	require.ErrorIs(t, err, ErrTeamIDRequired)
}

func TestAttendanceService_Search_CoachSeesAllTeam(t *testing.T) {
	expected := []dbs.Attendance{{ID: 1, TeamID: 5, UserID: 7}}
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) {
			assert.Equal(t, int64(5), teamID)
			return true, nil
		},
		isTeamOwnerFn: func(ctx *gin.Context, teamID, userID int64) (bool, error) {
			assert.Equal(t, int64(5), teamID)
			assert.Equal(t, int64(42), userID)
			return true, nil
		},
		searchFn: func(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
			require.NotNil(t, f.TeamID)
			assert.Equal(t, int64(5), *f.TeamID)
			assert.Nil(t, f.UserID)
			return expected, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	atts, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})

	require.NoError(t, err)
	assert.Equal(t, expected, atts)
}

func TestAttendanceService_Search_CoachByRoleWithoutOwnerRow(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		isTeamOwnerFn: func(ctx *gin.Context, teamID, userID int64) (bool, error) {
			return false, nil
		},
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleEntrenador), nil
		},
		searchFn: func(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
			require.NotNil(t, f.TeamID)
			assert.Nil(t, f.UserID)
			return nil, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})
	require.NoError(t, err)
}

func TestAttendanceService_Search_CoachFiltersByRunner(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleEntrenador), nil
		},
		searchFn: func(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
			require.NotNil(t, f.UserID)
			assert.Equal(t, int64(7), *f.UserID)
			return nil, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5), UserID: int64Ptr(7)})
	require.NoError(t, err)
}

func TestAttendanceService_Search_RunnerSeesOnlyOwn(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleCorredor), nil
		},
		searchFn: func(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
			require.NotNil(t, f.TeamID)
			require.NotNil(t, f.UserID)
			assert.Equal(t, int64(5), *f.TeamID)
			assert.Equal(t, int64(42), *f.UserID)
			return nil, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})
	require.NoError(t, err)
}

func TestAttendanceService_Search_RunnerCannotFilterAnotherRunner(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleCorredor), nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5), UserID: int64Ptr(7)})

	require.ErrorIs(t, err, ErrForbiddenAttendance)
}

func TestAttendanceService_Search_TeamWithSession(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleEntrenador), nil
		},
		searchFn: func(ctx *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
			require.NotNil(t, f.TeamID)
			require.NotNil(t, f.TrainingSessionID)
			assert.Equal(t, int64(5), *f.TeamID)
			assert.Equal(t, int64(9), *f.TrainingSessionID)
			return nil, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5), TrainingSessionID: int64Ptr(9)})
	require.NoError(t, err)
}

func TestAttendanceService_Search_TeamNotFound(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) {
			return false, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})

	require.ErrorIs(t, err, ErrTeamNotFound)
}

func TestAttendanceService_Search_NotMember_Forbidden(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) { return true, nil },
		getTeamUserRoleFn: func(ctx *gin.Context, teamID, userID int64) (string, error) {
			return "", nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})

	require.ErrorIs(t, err, ErrForbiddenAttendance)
}

// TestAttendanceService_Search_RegresionRolEntrenador fija el comportamiento de
// Search después de extraer teamRoleFor: los tres estados (no-miembro / corredor /
// entrenador) y, sobre todo, que el refactor no agregó consultas a la DB.
func TestAttendanceService_Search_RegresionRolEntrenador(t *testing.T) {
	teamID := int64(5)

	t.Run("no miembro del equipo recibe forbidden sin consultar el rol dos veces", func(t *testing.T) {
		mock := &mockAttendanceDao{
			teamExistsFn:      func(_ *gin.Context, id int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(_ *gin.Context, _, _ int64) (string, error) { return "", nil },
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

		_, err := svc.Search(nil, int64(99), attendance.SearchFilters{TeamID: &teamID})

		require.ErrorIs(t, err, ErrForbiddenAttendance)
		assert.Equal(t, 1, mock.called["IsTeamOwner"], "IsTeamOwner se consulta una sola vez")
		assert.Equal(t, 1, mock.called["GetTeamUserRole"], "GetTeamUserRole se consulta una sola vez")
	})

	t.Run("owner no consulta team_users porque puede no tener fila ahi", func(t *testing.T) {
		mock := &mockAttendanceDao{
			teamExistsFn:  func(_ *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn: func(_ *gin.Context, _, _ int64) (bool, error) { return true, nil },
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

		got, err := svc.Search(nil, int64(1), attendance.SearchFilters{TeamID: &teamID})

		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Equal(t, 0, mock.called["GetTeamUserRole"], "no debe consultar team_users para el owner")
	})

	t.Run("corredor queda forzado a su propio user_id", func(t *testing.T) {
		var gotFilters daos.AttendanceSearchFilters
		mock := &mockAttendanceDao{
			teamExistsFn:      func(_ *gin.Context, id int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(_ *gin.Context, _, _ int64) (string, error) { return string(constants.TeamUserRoleCorredor), nil },
			searchFn: func(_ *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
				gotFilters = f
				return nil, nil
			},
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

		_, err := svc.Search(nil, int64(12), attendance.SearchFilters{TeamID: &teamID})

		require.NoError(t, err)
		require.NotNil(t, gotFilters.UserID)
		assert.Equal(t, int64(12), *gotFilters.UserID)
	})

	t.Run("corredor no puede pedir asistencias de otro corredor", func(t *testing.T) {
		other := int64(13)
		mock := &mockAttendanceDao{
			teamExistsFn:      func(_ *gin.Context, id int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(_ *gin.Context, _, _ int64) (string, error) { return string(constants.TeamUserRoleCorredor), nil },
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

		_, err := svc.Search(nil, int64(12), attendance.SearchFilters{TeamID: &teamID, UserID: &other})

		require.ErrorIs(t, err, ErrForbiddenAttendance)
	})

	t.Run("entrenador ve todas las asistencias del equipo", func(t *testing.T) {
		target := int64(13)
		var gotFilters daos.AttendanceSearchFilters
		mock := &mockAttendanceDao{
			teamExistsFn:      func(_ *gin.Context, id int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(_ *gin.Context, _, _ int64) (string, error) { return string(constants.TeamUserRoleEntrenador), nil },
			searchFn: func(_ *gin.Context, f daos.AttendanceSearchFilters) ([]dbs.Attendance, error) {
				gotFilters = f
				return nil, nil
			},
		}
		svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

		_, err := svc.Search(nil, int64(7), attendance.SearchFilters{TeamID: &teamID, UserID: &target})

		require.NoError(t, err)
		require.NotNil(t, gotFilters.UserID)
		assert.Equal(t, int64(13), *gotFilters.UserID, "el entrenador puede filtrar por un corredor puntual")
	})
}

// validSessionCtx arma un contexto de sesión válido, para que cada test de
// resolveAttendanceSession rompa exactamente una condición.
func validSessionCtx() *daos.AttendanceSessionContext {
	from := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 24, 20, 0, 0, 0, time.UTC)
	location := `{"lat":-34.6,"lng":-58.4}`
	return &daos.AttendanceSessionContext{
		SessionInstanceID:  42,
		SessionName:        "Intervalos 800m",
		Date:               time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
		Kind:               string(constants.GroupCalendarDayKindTraining),
		IsPresencial:       true,
		PresencialTimeFrom: &from,
		PresencialTimeTo:   &to,
		PresencialLocation: &location,
		GroupID:            7,
		GroupName:          "Plan A",
		TeamID:             5,
		TeamName:           "Los Andes",
	}
}

func TestAttendanceService_ResolveAttendanceSession_Valid(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn:     func(_ *gin.Context, id int64) (bool, error) { return true, nil },
		findSessionCtxFn: func(_ *gin.Context, id int64) (*daos.AttendanceSessionContext, error) { return validSessionCtx(), nil },
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	got, err := svc.(*attendanceService).resolveAttendanceSession(nil, 5, 42)

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.SessionInstanceID)
	assert.Equal(t, int64(7), got.GroupID)
	assert.Equal(t, int64(5), got.TeamID)
}

func TestAttendanceService_ResolveAttendanceSession_Errors(t *testing.T) {
	tests := []struct {
		name    string
		teamOK  bool
		session *daos.AttendanceSessionContext
		wantErr error
	}{
		{
			name:    "equipo inexistente es 404",
			teamOK:  false,
			session: validSessionCtx(),
			wantErr: ErrTeamNotFound,
		},
		{
			name:    "sesión inexistente",
			teamOK:  true,
			session: nil,
			wantErr: ErrAttendanceSessionNotFound,
		},
		{
			name:   "sesión de un día que no es de entrenamiento",
			teamOK: true,
			session: func() *daos.AttendanceSessionContext {
				c := validSessionCtx()
				c.Kind = string(constants.GroupCalendarDayKindRest)
				return c
			}(),
			wantErr: ErrAttendanceSessionNotTraining,
		},
		{
			name:   "sesión cancelada",
			teamOK: true,
			session: func() *daos.AttendanceSessionContext {
				c := validSessionCtx()
				c.Kind = string(constants.GroupCalendarDayKindCancelled)
				return c
			}(),
			wantErr: ErrAttendanceSessionCancelled,
		},
		{
			name:   "sesión no presencial",
			teamOK: true,
			session: func() *daos.AttendanceSessionContext {
				c := validSessionCtx()
				c.IsPresencial = false
				return c
			}(),
			wantErr: ErrAttendanceSessionNotPresencial,
		},
		{
			name:   "sesión de otro equipo",
			teamOK: true,
			session: func() *daos.AttendanceSessionContext {
				c := validSessionCtx()
				c.TeamID = 6
				return c
			}(),
			wantErr: ErrAttendanceSessionWrongTeam,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := tt.session
			mock := &mockAttendanceDao{
				teamExistsFn:     func(_ *gin.Context, id int64) (bool, error) { return tt.teamOK, nil },
				findSessionCtxFn: func(_ *gin.Context, id int64) (*daos.AttendanceSessionContext, error) { return session, nil },
			}
			svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

			got, err := svc.(*attendanceService).resolveAttendanceSession(nil, 5, 42)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, got)
		})
	}
}

// TestAttendanceService_ResolveAttendanceSession_SesionFuturaNoEsError fija que
// la validación NO exige fecha pasada: el QR se emite para sesiones próximas.
func TestAttendanceService_ResolveAttendanceSession_SesionFuturaNoEsError(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(_ *gin.Context, id int64) (bool, error) { return true, nil },
		findSessionCtxFn: func(_ *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
			c := validSessionCtx()
			c.Date = time.Now().AddDate(0, 0, 7)
			return c, nil
		},
	}
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	got, err := svc.(*attendanceService).resolveAttendanceSession(nil, 5, 42)

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.SessionInstanceID)
}

// TestAttendanceService_Register_SourceQR fija la procedencia del alta por QR:
// la columna source es NOT NULL, así que el insert del escaneo del corredor tiene
// que mandarla siempre, junto con registered_by_user_id (spec: "el propio
// corredor").
func TestAttendanceService_Register_SourceQR(t *testing.T) {
	var got dbs.Attendance
	mock := registerMemberDao(func(ctx *gin.Context, a *dbs.Attendance) error {
		got = *a
		return nil
	})
	svc := NewAttendanceService(mock, nil, &attendanceGroupUserStub{}, nil, "http://localhost:8080")

	created, _, err := svc.Register(nil, 12, 5, 42)

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, string(constants.AttendanceSourceQR), got.Source)
	require.NotNil(t, got.RegisteredByUserID, "la columna registra quien cargo la fila: aca, el propio corredor")
	assert.Equal(t, int64(12), *got.RegisteredByUserID)
}

// attendanceUserStub implementa daos.UserDaoInterface. Solo se usa
// FindByIDs (el batch lookup de la grilla); el resto de métodos no lo invoca
// ningún handler de asistencia y devuelven un error explícito para que un uso
// futuro inesperado falle en el test en vez de devolver basura silenciosa.
type attendanceUserStub struct {
	users []*dbs.User
	err   error
	calls int
}

func (m *attendanceUserStub) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	m.calls++
	return m.users, m.err
}

func (m *attendanceUserStub) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) Update(ctx *gin.Context, user *dbs.User) error {
	return errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) UpdatePhoto(ctx *gin.Context, userID int64, key string, updatedAt time.Time) error {
	return errors.New("no usado en asistencia")
}
func (m *attendanceUserStub) ClearPhoto(ctx *gin.Context, userID int64) error {
	return errors.New("no usado en asistencia")
}

// groupStub implementa lo que la asistencia necesita de daos.GroupDaoInterface:
// FindByIDAndTeamID (validar que el grupo sea del equipo -> 404).
type groupStub struct {
	group *dbs.Group
	err   error
}

func (m *groupStub) FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
	return m.group, m.err
}

func (m *groupStub) Create(ctx *gin.Context, group *dbs.Group) error { return nil }
func (m *groupStub) FindByID(ctx *gin.Context, id int64) (*dbs.Group, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *groupStub) GetAll(ctx *gin.Context) ([]dbs.Group, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *groupStub) GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *groupStub) FindByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *groupStub) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
	return nil, errors.New("no usado en asistencia")
}
func (m *groupStub) Update(ctx *gin.Context, group *dbs.Group) error { return nil }
func (m *groupStub) SoftDelete(ctx *gin.Context, id int64) error     { return nil }
func (m *groupStub) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return nil
}

// coachDao arma un mockAttendanceDao donde el usuario 7 es owner (entrenador) del
// equipo 5, que es el caso feliz de autorización.
func coachDao(overrides func(m *mockAttendanceDao)) *mockAttendanceDao {
	m := &mockAttendanceDao{
		isTeamOwnerFn: func(_ *gin.Context, teamID, userID int64) (bool, error) { return true, nil },
		teamExistsFn:  func(_ *gin.Context, teamID int64) (bool, error) { return true, nil },
	}
	if overrides != nil {
		overrides(m)
	}
	return m
}

func TestAttendanceService_ListAttendanceSessions(t *testing.T) {
	teamID, groupID := int64(5), int64(7)

	t.Run("devuelve las sesiones con conteo y horario formateado", func(t *testing.T) {
		from := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
		to := time.Date(2026, 9, 24, 20, 0, 0, 0, time.UTC)
		dao := coachDao(func(m *mockAttendanceDao) {
			m.findPastSessionsFn = func(_ *gin.Context, gID, tID int64) ([]daos.SessionAttendanceOption, error) {
				assert.Equal(t, int64(7), gID)
				assert.Equal(t, int64(5), tID)
				return []daos.SessionAttendanceOption{
					{SessionInstanceID: 42, Name: "Intervalos", Date: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
						PresencialTimeFrom: &from, PresencialTimeTo: &to, AttendedCount: 3},
				}, nil
			}
		})
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, nil, "http://x")

		got, err := svc.ListAttendanceSessions(nil, 7, teamID, groupID)

		require.NoError(t, err)
		require.Len(t, got.Sessions, 1)
		assert.Equal(t, int64(42), got.Sessions[0].SessionInstanceID)
		assert.Equal(t, int64(3), got.Sessions[0].AttendedCount)
		require.NotNil(t, got.Sessions[0].PresencialTimeFrom)
		assert.Equal(t, "18:00", *got.Sessions[0].PresencialTimeFrom)
	})

	t.Run("grupo de otro equipo es 404", func(t *testing.T) {
		svc := NewAttendanceService(coachDao(nil), &groupStub{group: nil}, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.ListAttendanceSessions(nil, 7, teamID, groupID)

		require.ErrorIs(t, err, ErrAttendanceGroupNotFound)
	})

	t.Run("un corredor del equipo no puede listar", func(t *testing.T) {
		dao := coachDao(func(m *mockAttendanceDao) {
			m.isTeamOwnerFn = func(_ *gin.Context, teamID, userID int64) (bool, error) { return false, nil }
			m.getTeamUserRoleFn = func(_ *gin.Context, teamID, userID int64) (string, error) {
				return string(constants.TeamUserRoleCorredor), nil
			}
		})
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, nil, "http://x")

		_, err := svc.ListAttendanceSessions(nil, 12, teamID, groupID)

		require.ErrorIs(t, err, ErrForbiddenAttendance)
	})
}

func TestAttendanceService_GetSessionAttendance(t *testing.T) {
	teamID, groupID, sessionID := int64(5), int64(7), int64(42)

	rosterWith := func(rows []daos.SessionAttendanceRow, agg *daos.SessionAttendanceAggregates) func(*mockAttendanceDao) {
		return func(m *mockAttendanceDao) {
			m.findSessionCtxFn = func(_ *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
				return validSessionCtx(), nil
			}
			m.findRosterFn = func(_ *gin.Context, g, t, s int64, d time.Time) ([]daos.SessionAttendanceRow, *daos.SessionAttendanceAggregates, error) {
				return rows, agg, nil
			}
		}
	}
	users := []*dbs.User{
		{ID: 12, Name: "Ana", Surname: "Gomez", Email: "ana@mail.com"},
		{ID: 13, Name: "Zoe", Surname: "Diaz", Email: "zoe@mail.com"},
	}
	svcWith := func(dao *mockAttendanceDao, users []*dbs.User) AttendanceServiceInterface {
		return NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, &attendanceUserStub{users: users}, "http://x")
	}

	t.Run("grilla completa con summary y orden alfabetico", func(t *testing.T) {
		attID := int64(88)
		source := string(constants.AttendanceSourceManual)
		registered := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
		rows := []daos.SessionAttendanceRow{
			{UserID: 13}, // sin asistencia
			{UserID: 12, AttendanceID: &attID, Source: &source, RegisteredAt: &registered},
		}
		agg := &daos.SessionAttendanceAggregates{RosterSize: 2, Attended: 1}
		svc := svcWith(coachDao(rosterWith(rows, agg)), users)

		got, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, int64(2), got.Summary.RosterSize)
		assert.Equal(t, int64(1), got.Summary.Attended)
		assert.Equal(t, int64(1), got.Summary.NotConfirmed)
		assert.Equal(t, 50.0, got.Summary.AttendanceRatePct)
		require.Len(t, got.Roster, 2)
		// orden alfabetico: Ana antes que Zoe
		assert.Equal(t, int64(12), got.Roster[0].UserID)
		assert.Equal(t, "Ana Gomez", got.Roster[0].Name)
		assert.Equal(t, attendance.SessionAttendanceStatusAttended, got.Roster[0].Status)
		require.NotNil(t, got.Roster[0].Source)
		assert.Equal(t, "manual", *got.Roster[0].Source)
		assert.Equal(t, int64(13), got.Roster[1].UserID)
		assert.Equal(t, attendance.SessionAttendanceStatusNotConfirmed, got.Roster[1].Status)
		assert.Nil(t, got.Roster[1].Source, "sin asistencia no puede traer source")
		assert.Nil(t, got.Roster[1].AttendanceID)
		assert.Equal(t, int64(7), got.Session.GroupID)
		assert.Equal(t, "Plan A", got.Session.GroupName)
	})

	t.Run("roster vacio no divide por cero", func(t *testing.T) {
		svc := svcWith(coachDao(rosterWith(nil, &daos.SessionAttendanceAggregates{RosterSize: 0, Attended: 0})), nil)

		got, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, 0.0, got.Summary.AttendanceRatePct)
		assert.Empty(t, got.Roster)
	})

	t.Run("attended mayor que roster no da not_confirmed negativo", func(t *testing.T) {
		rows := []daos.SessionAttendanceRow{{UserID: 12}}
		agg := &daos.SessionAttendanceAggregates{RosterSize: 1, Attended: 3}
		svc := svcWith(coachDao(rosterWith(rows, agg)), users)

		got, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, int64(0), got.Summary.NotConfirmed)
	})

	t.Run("rate redondeado a un decimal", func(t *testing.T) {
		rows := []daos.SessionAttendanceRow{{UserID: 12}}
		svc := svcWith(coachDao(rosterWith(rows, &daos.SessionAttendanceAggregates{RosterSize: 3, Attended: 1})), users)

		got, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, 33.3, got.Summary.AttendanceRatePct)
	})

	t.Run("group_id que no es el de la sesion es 422", func(t *testing.T) {
		svc := svcWith(coachDao(rosterWith(nil, nil)), users)

		_, err := svc.GetSessionAttendance(nil, 7, teamID, int64(999), sessionID)

		require.ErrorIs(t, err, ErrAttendanceGroupMismatch)
	})

	t.Run("corredor no puede ver la grilla", func(t *testing.T) {
		dao := coachDao(rosterWith(nil, nil))
		dao.isTeamOwnerFn = func(_ *gin.Context, teamID, userID int64) (bool, error) { return false, nil }
		dao.getTeamUserRoleFn = func(_ *gin.Context, teamID, userID int64) (string, error) {
			return string(constants.TeamUserRoleCorredor), nil
		}
		svc := svcWith(dao, users)

		_, err := svc.GetSessionAttendance(nil, 12, teamID, groupID, sessionID)

		require.ErrorIs(t, err, ErrForbiddenAttendance)
	})

	t.Run("sesion asincronica es 422 y no consulta el roster", func(t *testing.T) {
		dao := coachDao(func(m *mockAttendanceDao) {
			m.findSessionCtxFn = func(_ *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
				c := validSessionCtx()
				c.IsPresencial = false
				return c, nil
			}
		})
		svc := svcWith(dao, users)

		_, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.ErrorIs(t, err, ErrAttendanceSessionNotPresencial)
		assert.Equal(t, 0, dao.called["FindGroupRosterWithAttendance"], "no debe consultar la grilla si la sesion no es valida")
	})

	t.Run("resuelve los nombres con una sola llamada al batch", func(t *testing.T) {
		rows := []daos.SessionAttendanceRow{{UserID: 12}, {UserID: 13}}
		userStub := &attendanceUserStub{users: users}
		svc := NewAttendanceService(coachDao(rosterWith(rows, &daos.SessionAttendanceAggregates{RosterSize: 2})),
			&groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, userStub, "http://x")

		_, err := svc.GetSessionAttendance(nil, 7, teamID, groupID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, 1, userStub.calls, "una sola query para todos los corredores, no una por fila")
	})
}

// TestAttendanceService_ComposeUserName cubre la composicion de nombres de la
// grilla: tiene que usar EXACTAMENTE la misma regla que el roster de la app, o
// el mismo corredor apareceria escrito de dos formas distintas.
func TestAttendanceService_ComposeUserName(t *testing.T) {
	tests := []struct {
		name string
		user dbs.User
		want string
	}{
		{"nombre y apellido", dbs.User{Name: "Ana", Surname: "Gomez", Email: "a@m.com"}, "Ana Gomez"},
		// Los espacios de los extremos se recortan, pero los INTERNOS se conservan
		// a proposito: el roster del front hace `${name} ${surname}`.trim()
		// (hooks/use-team-roster.js:60) y tampoco colapsa los internos. Colapsarlos
		// aca haria que la grilla y el roster escribieran distinto al mismo
		// corredor, que es justo lo que la spec quiere evitar.
		{"espacios de los extremos se recortan", dbs.User{Name: "  Ana ", Surname: "  Gomez  ", Email: "a@m.com"}, "Ana    Gomez"},
		{"solo nombre", dbs.User{Name: "Ana", Surname: "", Email: "a@m.com"}, "Ana"},
		{"solo apellido", dbs.User{Name: "", Surname: "Gomez", Email: "a@m.com"}, "Gomez"},
		{"ambos vacios cae al email", dbs.User{Name: "   ", Surname: "  ", Email: "a@m.com"}, "a@m.com"},
		{"ambos nil cae al email", dbs.User{Name: "", Surname: "", Email: "a@m.com"}, "a@m.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tt.user
			assert.Equal(t, tt.want, composeUserName(&u))
		})
	}
}

// attendanceGroupUserStub implementa lo que la asistencia le pide al
// GroupUserDaoInterface: la membresía evaluada contra la fecha de la sesión (D7).
// Por default responde que todo el mundo es miembro, que es el caso feliz de la
// mayoría de los tests de lectura, que no llegan a tocar estos métodos.
type attendanceGroupUserStub struct {
	// Se embebe el mockGroupUserDao que ya existe en el paquete para no
	// implementar a mano los ~15 métodos de la interfaz: la asistencia solo
	// llama a los dos de abajo, que quedan shadoweados por los métodos propios
	// de este struct. Los demás nunca se invocan desde acá.
	mockGroupUserDao

	isMemberFn   func(ctx *gin.Context, groupID, userID int64, sessionDate time.Time) (bool, error)
	missingFn    func(ctx *gin.Context, groupID int64, userIDs []int64, sessionDate time.Time) ([]int64, error)
	lastDate     time.Time
	lastGroupIDs []int64
}

func (m *attendanceGroupUserStub) IsActiveGroupMember(ctx *gin.Context, groupID, userID int64, sessionDate time.Time) (bool, error) {
	m.lastDate = sessionDate
	m.lastGroupIDs = append(m.lastGroupIDs, groupID)
	if m.isMemberFn != nil {
		return m.isMemberFn(ctx, groupID, userID, sessionDate)
	}
	return true, nil
}

func (m *attendanceGroupUserStub) MissingGroupMembers(ctx *gin.Context, groupID int64, userIDs []int64, sessionDate time.Time) ([]int64, error) {
	m.lastDate = sessionDate
	m.lastGroupIDs = append(m.lastGroupIDs, groupID)
	if m.missingFn != nil {
		return m.missingFn(ctx, groupID, userIDs, sessionDate)
	}
	return nil, nil
}

func (m *mockAttendanceDao) BulkUpsertManual(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error) {
	if m.bulkUpsertFn != nil {
		return m.bulkUpsertFn(ctx, teamID, sessionInstanceID, actorUserID, userIDs)
	}
	return len(userIDs), 0, nil
}

func (m *mockAttendanceDao) FindByID(ctx *gin.Context, attendanceID int64) (*dbs.Attendance, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, attendanceID)
	}
	return nil, nil
}

func (m *mockAttendanceDao) DeleteByID(ctx *gin.Context, attendanceID, teamID int64) (bool, error) {
	if m.deleteByIDFn != nil {
		return m.deleteByIDFn(ctx, attendanceID, teamID)
	}
	return true, nil
}

// sessionCtxFor arma el contexto de sesión que espera resolveAttendanceSession para
// un grupo/equipo/sesión válidos y no cancelados.
func sessionCtxFor(groupID, teamID int64, date time.Time) *daos.AttendanceSessionContext {
	return &daos.AttendanceSessionContext{
		TeamID:       teamID,
		GroupID:      groupID,
		Date:         date,
		Kind:         "training",
		IsPresencial: true,
	}
}

func TestAttendanceService_BulkSaveAttendance(t *testing.T) {
	const (
		teamID    = int64(5)
		groupID   = int64(7)
		sessionID = int64(42)
		coachID   = int64(3)
	)
	sessionDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)

	daoFor := func(bulk func(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error)) *mockAttendanceDao {
		return &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return userID == coachID, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
			findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
				return sessionCtxFor(groupID, teamID, sessionDate), nil
			},
			bulkUpsertFn: bulk,
		}
	}

	t.Run("lote valido devuelve los contadores del dao", func(t *testing.T) {
		var gotActor int64
		var gotUserIDs []int64
		dao := daoFor(func(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error) {
			gotActor = actorUserID
			gotUserIDs = userIDs
			return 2, 1, nil
		})
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		got, err := svc.BulkSaveAttendance(nil, coachID, teamID, sessionID, []int64{12, 13, 14})

		require.NoError(t, err)
		assert.Equal(t, 2, got.Created)
		assert.Equal(t, 1, got.Updated)
		assert.Equal(t, coachID, gotActor, "registered_by_user_id es el entrenador que carga, no el corredor")
		assert.Equal(t, []int64{12, 13, 14}, gotUserIDs)
	})

	t.Run("un user_id ajeno al grupo invalida todo el lote y no escribe", func(t *testing.T) {
		wrote := false
		dao := daoFor(func(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error) {
			wrote = true
			return 0, 0, nil
		})
		gu := &attendanceGroupUserStub{
			missingFn: func(ctx *gin.Context, groupID int64, userIDs []int64, sessionDate time.Time) ([]int64, error) {
				return []int64{999}, nil
			},
		}
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, gu, &attendanceUserStub{}, "http://x")

		got, err := svc.BulkSaveAttendance(nil, coachID, teamID, sessionID, []int64{12, 999})

		require.Error(t, err)
		assert.Nil(t, got, "no puede devolver contadores si rechazó el lote")
		assert.False(t, wrote, "todo-o-nada: con un invalido no se escribe NINGUNA fila, ni las validas")
		assert.True(t, errors.Is(err, ErrAttendanceBulkInvalidUsers), "el controller mapea este error a 422")

		var bulkErr *ErrBulkInvalidUsers
		require.True(t, errors.As(err, &bulkErr))
		assert.Equal(t, []int64{999}, bulkErr.UserIDs, "el 422 tiene que listar los user_id rechazados, no decir solo 'algunos son invalidos'")
	})

	t.Run("la membresia se evalua contra la fecha de la sesion, no contra hoy", func(t *testing.T) {
		gu := &attendanceGroupUserStub{}
		dao := daoFor(func(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (int, int, error) {
			return 1, 0, nil
		})
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, gu, &attendanceUserStub{}, "http://x")

		_, err := svc.BulkSaveAttendance(nil, coachID, teamID, sessionID, []int64{12})

		require.NoError(t, err)
		assert.Equal(t, sessionDate, gu.lastDate, "D7: la membresía se mira en el día de la sesión")
		assert.Equal(t, []int64{groupID}, gu.lastGroupIDs, "la membresía se consulta contra el grupo de la sesión, no el del body")
	})

	t.Run("un no-entrenador no carga", func(t *testing.T) {
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return false, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
		}
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		_, err := svc.BulkSaveAttendance(nil, int64(77), teamID, sessionID, []int64{12})

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrForbiddenAttendance))
	})

	t.Run("sesion no presencial es 422 y no escribe", func(t *testing.T) {
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return userID == coachID, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
			findSessionCtxFn: func(ctx *gin.Context, id int64) (*daos.AttendanceSessionContext, error) {
				ctxRet := sessionCtxFor(groupID, teamID, sessionDate)
				ctxRet.IsPresencial = false
				return ctxRet, nil
			},
		}
		svc := NewAttendanceService(dao, &groupStub{group: &dbs.Group{ID: groupID, TeamID: teamID}}, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		_, err := svc.BulkSaveAttendance(nil, coachID, teamID, sessionID, []int64{12})

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAttendanceSessionNotPresencial))
	})
}

func TestAttendanceService_DeleteAttendance(t *testing.T) {
	const (
		teamID       = int64(5)
		coachID      = int64(3)
		attendanceID = int64(88)
	)

	t.Run("borra y devuelve nil", func(t *testing.T) {
		var deletedID, deletedTeam int64
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return userID == coachID, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
			findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Attendance, error) {
				return &dbs.Attendance{ID: attendanceID, TeamID: teamID}, nil
			},
			deleteByIDFn: func(ctx *gin.Context, id, team int64) (bool, error) {
				deletedID, deletedTeam = id, team
				return true, nil
			},
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		err := svc.DeleteAttendance(nil, coachID, teamID, attendanceID)

		require.NoError(t, err)
		assert.Equal(t, attendanceID, deletedID)
		assert.Equal(t, teamID, deletedTeam)
	})

	t.Run("asistencia inexistente es 404", func(t *testing.T) {
		dao := &mockAttendanceDao{
			findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Attendance, error) { return nil, nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		err := svc.DeleteAttendance(nil, coachID, teamID, attendanceID)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAttendanceNotFound))
	})

	t.Run("asistencia de otro equipo es 403 y no se consulta el rol", func(t *testing.T) {
		roleChecked := false
		dao := &mockAttendanceDao{
			findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Attendance, error) {
				return &dbs.Attendance{ID: attendanceID, TeamID: 6}, nil
			},
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { roleChecked = true; return true, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { roleChecked = true; return "entrenador", nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		err := svc.DeleteAttendance(nil, coachID, teamID, attendanceID)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAttendanceForbiddenTeam))
		assert.False(t, roleChecked, "si ya sabemos que es de otro equipo, no hay nada que autorizar")
	})

	t.Run("no-entrenador del equipo es 403", func(t *testing.T) {
		deleted := false
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return false, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
			findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Attendance, error) {
				return &dbs.Attendance{ID: attendanceID, TeamID: teamID}, nil
			},
			deleteByIDFn: func(ctx *gin.Context, id, team int64) (bool, error) { deleted = true; return true, nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		err := svc.DeleteAttendance(nil, int64(77), teamID, attendanceID)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAttendanceForbiddenTeam))
		assert.False(t, deleted, "un no-entrenador no borra nada")
	})

	t.Run("otro request borro la fila entre FindByID y DELETE: 404", func(t *testing.T) {
		dao := &mockAttendanceDao{
			teamExistsFn:      func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
			isTeamOwnerFn:     func(ctx *gin.Context, id, userID int64) (bool, error) { return true, nil },
			getTeamUserRoleFn: func(ctx *gin.Context, id, userID int64) (string, error) { return "", nil },
			findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Attendance, error) {
				return &dbs.Attendance{ID: attendanceID, TeamID: teamID}, nil
			},
			deleteByIDFn: func(ctx *gin.Context, id, team int64) (bool, error) { return false, nil },
		}
		svc := NewAttendanceService(dao, nil, &attendanceGroupUserStub{}, &attendanceUserStub{}, "http://x")

		err := svc.DeleteAttendance(nil, coachID, teamID, attendanceID)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAttendanceNotFound))
	})
}
