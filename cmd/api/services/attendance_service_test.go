package services

import (
	"errors"
	"testing"

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
	if m.getTeamUserRoleFn != nil {
		return m.getTeamUserRoleFn(ctx, teamID, userID)
	}
	return "", nil
}

func TestAttendanceService_GenerateQR_DeterministicAndURL(t *testing.T) {
	svc := NewAttendanceService(&mockAttendanceDao{}, "http://localhost:8080")

	qr1, err := svc.GenerateQR(nil, 5, 9)
	require.NoError(t, err)
	qr2, err := svc.GenerateQR(nil, 5, 9)
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/api/v1/attendance/team/5/session/9", qr1.URLEncoded)
	assert.Equal(t, qr1.QRCodeBase64, qr2.QRCodeBase64)
	assert.NotEmpty(t, qr1.QRCodeBase64)
}

func TestAttendanceService_GenerateQR_BaseURLWithoutTrailingSlash(t *testing.T) {
	svc := NewAttendanceService(&mockAttendanceDao{}, "http://localhost:8080/")

	qr, err := svc.GenerateQR(nil, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/api/v1/attendance/team/1/session/2", qr.URLEncoded)
}

func TestAttendanceService_Register_Created(t *testing.T) {
	mock := &mockAttendanceDao{
		createFn: func(ctx *gin.Context, a *dbs.Attendance) error {
			assert.Equal(t, int64(7), a.UserID)
			assert.Equal(t, int64(5), a.TeamID)
			assert.Equal(t, int64(9), a.TrainingSessionID)
			return nil
		},
	}
	svc := NewAttendanceService(mock, "http://localhost:8080")

	created, err := svc.Register(nil, 7, 5, 9)

	require.NoError(t, err)
	assert.True(t, created)
}

func TestAttendanceService_Register_AlreadyExists(t *testing.T) {
	mock := &mockAttendanceDao{
		createFn: func(ctx *gin.Context, a *dbs.Attendance) error {
			return daos.ErrAttendanceAlreadyExists
		},
	}
	svc := NewAttendanceService(mock, "http://localhost:8080")

	created, err := svc.Register(nil, 7, 5, 9)

	require.NoError(t, err)
	assert.False(t, created)
}

func TestAttendanceService_Register_DAOError(t *testing.T) {
	mock := &mockAttendanceDao{
		createFn: func(ctx *gin.Context, a *dbs.Attendance) error {
			return errors.New("db caída")
		},
	}
	svc := NewAttendanceService(mock, "http://localhost:8080")

	created, err := svc.Register(nil, 7, 5, 9)

	require.Error(t, err)
	assert.False(t, created)
}

func TestAttendanceService_Search_TeamIDRequired(t *testing.T) {
	mock := &mockAttendanceDao{}
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5), TrainingSessionID: int64Ptr(9)})
	require.NoError(t, err)
}

func TestAttendanceService_Search_TeamNotFound(t *testing.T) {
	mock := &mockAttendanceDao{
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) {
			return false, nil
		},
	}
	svc := NewAttendanceService(mock, "http://localhost:8080")

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
	svc := NewAttendanceService(mock, "http://localhost:8080")

	_, err := svc.Search(nil, 42, attendance.SearchFilters{TeamID: int64Ptr(5)})

	require.ErrorIs(t, err, ErrForbiddenAttendance)
}
