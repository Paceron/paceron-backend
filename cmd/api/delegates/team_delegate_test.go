package delegates

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/group"
	"simple-arq-golang/cmd/api/domains/team"
)

type mockTeamServiceForDelegate struct {
	createFn func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error)
}

func (m *mockTeamServiceForDelegate) Create(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, ownerID, req)
	}
	return nil, nil
}
func (m *mockTeamServiceForDelegate) Update(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
	return nil, nil
}
func (m *mockTeamServiceForDelegate) Delete(ctx *gin.Context, id int64, userID int64) error {
	return nil
}
func (m *mockTeamServiceForDelegate) GetByID(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
	return nil, nil
}
func (m *mockTeamServiceForDelegate) GetAll(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
	return nil, nil
}
func (m *mockTeamServiceForDelegate) UpdateAddress(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
	return nil, nil
}

func (m *mockTeamServiceForDelegate) UploadIcon(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
	return nil, nil
}

func (m *mockTeamServiceForDelegate) DeleteIcon(ctx *gin.Context, id int64, callerID int64) error {
	return nil
}

type mockGroupServiceForDelegate struct {
	createFn        func(ctx *gin.Context, callerID int64, req *group.CreateGroupRequest) (*group.GroupResponse, error)
	createCalled    bool
	lastCreateReq   *group.CreateGroupRequest
	lastCreateOwner int64
}

func (m *mockGroupServiceForDelegate) Create(ctx *gin.Context, callerID int64, req *group.CreateGroupRequest) (*group.GroupResponse, error) {
	m.createCalled = true
	m.lastCreateReq = req
	m.lastCreateOwner = callerID
	if m.createFn != nil {
		return m.createFn(ctx, callerID, req)
	}
	return &group.GroupResponse{ID: 1, Name: req.Name, TeamID: req.TeamID, IsMain: req.IsMain}, nil
}
func (m *mockGroupServiceForDelegate) Update(ctx *gin.Context, id int64, callerID int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error) {
	return nil, nil
}
func (m *mockGroupServiceForDelegate) Delete(ctx *gin.Context, id int64, userID int64) error {
	return nil
}
func (m *mockGroupServiceForDelegate) GetByID(ctx *gin.Context, id int64) (*group.GroupResponse, error) {
	return nil, nil
}
func (m *mockGroupServiceForDelegate) GetAll(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
	return nil, nil
}

func TestTeamDelegate_CreateTeam_CreatesDefaultGroupWhenFlagOmitted(t *testing.T) {
	teamSvc := &mockTeamServiceForDelegate{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Name: req.Name}, nil
		},
	}
	groupSvc := &mockGroupServiceForDelegate{}

	d := NewTeamDelegate(teamSvc, groupSvc)
	_, err := d.CreateTeam(nil, 1, &team.CreateTeamRequest{Name: "Equipo Alpha"})

	assert.NoError(t, err)
	assert.True(t, groupSvc.createCalled)
	assert.True(t, groupSvc.lastCreateReq.IsMain)
	assert.Equal(t, int64(1), groupSvc.lastCreateOwner)
}

func TestTeamDelegate_CreateTeam_CreatesDefaultGroupWhenTrue(t *testing.T) {
	teamSvc := &mockTeamServiceForDelegate{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Name: req.Name}, nil
		},
	}
	groupSvc := &mockGroupServiceForDelegate{}

	d := NewTeamDelegate(teamSvc, groupSvc)
	createDefault := true
	_, err := d.CreateTeam(nil, 1, &team.CreateTeamRequest{Name: "Equipo Alpha", CreateDefaultGroup: &createDefault})

	assert.NoError(t, err)
	assert.True(t, groupSvc.createCalled)
}

func TestTeamDelegate_CreateTeam_SkipsDefaultGroupWhenExplicitFalse(t *testing.T) {
	teamSvc := &mockTeamServiceForDelegate{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Name: req.Name}, nil
		},
	}
	groupSvc := &mockGroupServiceForDelegate{}

	d := NewTeamDelegate(teamSvc, groupSvc)
	createDefault := false
	_, err := d.CreateTeam(nil, 1, &team.CreateTeamRequest{Name: "Equipo Alpha", CreateDefaultGroup: &createDefault})

	assert.NoError(t, err)
	assert.False(t, groupSvc.createCalled)
}

func TestTeamDelegate_CreateTeam_TeamCreateError(t *testing.T) {
	teamSvc := &mockTeamServiceForDelegate{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return nil, assert.AnError
		},
	}
	groupSvc := &mockGroupServiceForDelegate{}

	d := NewTeamDelegate(teamSvc, groupSvc)
	_, err := d.CreateTeam(nil, 1, &team.CreateTeamRequest{Name: "Equipo Alpha"})

	assert.Error(t, err)
	assert.False(t, groupSvc.createCalled)
}
