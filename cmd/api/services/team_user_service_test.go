package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamuser"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

// callerID usado en los tests de AddUser/RemoveUser cuando el llamante es el entrenador
// (distinto del target user, para poder distinguir ambos roles en el mismo mock).
const testEntrenadorCallerID int64 = 100

type mockTeamUserDao struct {
	createFn                         func(ctx *gin.Context, tu *dbs.TeamUser) error
	findByTeamAndUserFn              func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error)
	findByTeamIDFn                   func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error)
	findByUserIDFn                   func(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error)
	countActiveByTeamFn              func(ctx *gin.Context, teamID int64) (int64, error)
	countActiveByTeamExcludingUserFn func(ctx *gin.Context, teamID, excludeUserID int64) (int64, error)
	hasOwnerByTeamFn                 func(ctx *gin.Context, teamID int64) (bool, error)
	softDeleteFn                     func(ctx *gin.Context, id int64) error
	softDeleteByTeamIDFn             func(ctx *gin.Context, teamID int64) error
}

func (m *mockTeamUserDao) Create(ctx *gin.Context, tu *dbs.TeamUser) error {
	if m.createFn != nil {
		return m.createFn(ctx, tu)
	}
	return nil
}

func (m *mockTeamUserDao) FindByTeamAndUser(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
	if m.findByTeamAndUserFn != nil {
		return m.findByTeamAndUserFn(ctx, teamID, userID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) FindByTeamID(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
	if m.findByTeamIDFn != nil {
		return m.findByTeamIDFn(ctx, teamID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) CountActiveByTeam(ctx *gin.Context, teamID int64) (int64, error) {
	if m.countActiveByTeamFn != nil {
		return m.countActiveByTeamFn(ctx, teamID)
	}
	return 0, nil
}

func (m *mockTeamUserDao) CountActiveByTeamExcludingUser(ctx *gin.Context, teamID, excludeUserID int64) (int64, error) {
	if m.countActiveByTeamExcludingUserFn != nil {
		return m.countActiveByTeamExcludingUserFn(ctx, teamID, excludeUserID)
	}
	return 0, nil
}

func (m *mockTeamUserDao) HasOwnerByTeam(ctx *gin.Context, teamID int64) (bool, error) {
	if m.hasOwnerByTeamFn != nil {
		return m.hasOwnerByTeamFn(ctx, teamID)
	}
	return false, nil
}

func (m *mockTeamUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockTeamUserDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	if m.softDeleteByTeamIDFn != nil {
		return m.softDeleteByTeamIDFn(ctx, teamID)
	}
	return nil
}

// entrenadorCallerFindByTeamAndUser devuelve al testEntrenadorCallerID como entrenador,
// y delega el resto de las consultas (ej. el target user de AddUser/RemoveUser) a other.
func entrenadorCallerFindByTeamAndUser(other func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error)) func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
	return func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
		if userID == testEntrenadorCallerID {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "entrenador"}, nil
		}
		if other != nil {
			return other(ctx, teamID, userID)
		}
		return nil, nil
	}
}

func TestTeamUserService_AddUser_Success(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			tu.ID = 1
			return nil
		},
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 2, nil
		},
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "corredor", resp.RoleInTeam)
}

func TestTeamUserService_AddUser_NotEntrenador(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "corredor"}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, 2, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede agregar usuarios al equipo")
}

func TestTeamUserService_AddUser_AssignsToMainGroup(t *testing.T) {
	groupUserCreateCalled := false
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			tu.ID = 1
			return nil
		},
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}
	mockGroup := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 9, TeamID: 1, IsMain: true}}, nil
		},
	}
	mockGroupUser := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			groupUserCreateCalled = true
			assert.Equal(t, int64(9), gu.GroupID)
			assert.Equal(t, int64(1), gu.UserID)
			return nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, mockGroup, mockGroupUser, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.NoError(t, err)
	assert.True(t, groupUserCreateCalled)
}

func TestTeamUserService_AddUser_NoMainGroup_StillSucceeds(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			tu.ID = 1
			return nil
		},
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}
	mockGroup := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, mockGroup, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestTeamUserService_AddUser_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 999, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_AddUser_InvalidRole(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 20}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "User"}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "entrenador",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol inválido")
}

func TestTeamUserService_AddUser_UserNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     999,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestTeamUserService_AddUser_AlreadyMember(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1}, nil
		}),
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario ya pertenece a este equipo")
}

func TestTeamUserService_AddUser_MaxMembersReached(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 10, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "máximo de 10 miembros")
}

func TestTeamUserService_RemoveUser_Self(t *testing.T) {
	softDeleteCalled := false
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 1, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestTeamUserService_RemoveUser_EntrenadorRemovesOther(t *testing.T) {
	softDeleteCalled := false
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		}),
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, testEntrenadorCallerID, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

// TestTeamUserService_RemoveUser_NotifiesRemovedUser cubre el trigger de expulsión:
// el corredor removido recibe mail + push cuando el entrenador lo saca del equipo.
func TestTeamUserService_RemoveUser_NotifiesRemovedUser(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		}),
		softDeleteFn: func(ctx *gin.Context, id int64) error { return nil },
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Los Pumas", OwnerID: testEntrenadorCallerID}, nil
		},
	}
	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	mailerMock := &mockMailer{}
	pushClient := &mockExpoPushClient{}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: userID, Token: "ExponentPushToken[juan]"}}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, pushTokenDao, pushClient)
	err := svc.RemoveUser(nil, 1, testEntrenadorCallerID, 7)

	require.NoError(t, err)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "juan@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeTeamRemoved, mailerMock.lastEmailType)
	assert.Equal(t, "Los Pumas", mailerMock.lastData.TeamName)
	assert.Equal(t, 1, pushClient.sendCallCount)
	assert.Equal(t, "ExponentPushToken[juan]", pushClient.lastToken)
	assert.Equal(t, "team_removed", pushClient.lastData["type"])
	assert.Equal(t, "/teams", pushClient.lastData["route"])
}

// TestTeamUserService_RemoveUser_NotifiesOwnerOnSelfLeave cubre el trigger de
// "corredor deja el equipo": el entrenador recibe mail + push cuando un corredor
// se saca a sí mismo.
func TestTeamUserService_RemoveUser_NotifiesOwnerOnSelfLeave(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error { return nil },
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Los Pumas", OwnerID: 99}, nil
		},
	}
	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			if userID == 99 {
				return &dbs.User{ID: 99, Name: "Coach", Email: "coach@test.com"}, nil
			}
			return &dbs.User{ID: userID, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	mailerMock := &mockMailer{}
	pushClient := &mockExpoPushClient{}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: 99, Token: "ExponentPushToken[coach]"}}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, pushTokenDao, pushClient)
	err := svc.RemoveUser(nil, 1, 7, 7)

	require.NoError(t, err)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "coach@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeTeamMemberLeft, mailerMock.lastEmailType)
	assert.Equal(t, "Juan", mailerMock.lastData.RelatedUserName)
	assert.Equal(t, 1, pushClient.sendCallCount)
	assert.Equal(t, "ExponentPushToken[coach]", pushClient.lastToken)
	assert.Equal(t, "team_member_left", pushClient.lastData["type"])
	assert.Equal(t, "/teams/1", pushClient.lastData["route"])
}

func TestTeamUserService_RemoveUser_NotSelfNotEntrenador(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 2, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede quitar a otro usuario del equipo")
}

func TestTeamUserService_RemoveUser_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 999, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_RemoveUser_NotMember(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 999, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este equipo")
}

func TestTeamUserService_RemoveUser_CannotRemoveOwner(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "entrenador"}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se puede quitar al entrenador")
}

func TestTeamUserService_AddUser_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_UserFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_FindByTeamAndUserError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		}),
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_CreateError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 2, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			return errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_RemoveUser_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_RemoveUser_FindByTeamAndUserError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_RemoveUser_SoftDeleteError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.RemoveUser(nil, 1, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_AddUser_CountError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil),
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 0, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.AddUser(nil, 1, testEntrenadorCallerID, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_GetUsersByTeam_Success(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{
				{ID: 1, TeamID: 1, UserID: 10, RoleInTeam: "entrenador", Status: "active"},
				{ID: 2, TeamID: 1, UserID: 20, RoleInTeam: "corredor", Status: "active"},
			}, nil
		},
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.GetUsersByTeam(nil, 1, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, int64(10), resp[0].UserID)
	assert.Equal(t, "entrenador", resp[0].RoleInTeam)
}

func TestTeamUserService_GetUsersByTeam_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.GetUsersByTeam(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_GetUsersByTeam_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.GetUsersByTeam(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del equipo")
}

func TestTeamUserService_GetUsersByTeam_NotMember(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.GetUsersByTeam(nil, 1, 99)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este equipo")
}

func TestTeamUserService_GetUsersByTeam_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.GetUsersByTeam(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del equipo")
}

func TestTeamUserService_GetUsersByTeam_FindByTeamIDError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.GetUsersByTeam(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del equipo")
}

func TestTeamUserService_GetUsersByTeam_Empty(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{}, nil
		},
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.GetUsersByTeam(nil, 1, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 0)
}
