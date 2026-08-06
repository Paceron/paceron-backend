package delegates

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/group"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

// TeamDelegate coordina operaciones que involucran teams y groups.
type TeamDelegate interface {
	CreateTeam(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error)
}

type teamDelegate struct {
	teamSvc  services.TeamServiceInterface
	groupSvc services.GroupServiceInterface
}

// NewTeamDelegate crea una nueva instancia de TeamDelegate.
func NewTeamDelegate(teamSvc services.TeamServiceInterface, groupSvc services.GroupServiceInterface) TeamDelegate {
	return &teamDelegate{
		teamSvc:  teamSvc,
		groupSvc: groupSvc,
	}
}

// CreateTeam crea un equipo y su grupo principal. La membresía de equipo siempre pasa
// por un grupo (el principal u otro más específico), así que el grupo principal se crea
// por default — create_default_group solo sirve para saltearlo, pasando explícitamente
// false.
func (d *teamDelegate) CreateTeam(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	teamResp, err := d.teamSvc.Create(ctx, ownerID, req)
	if err != nil {
		return nil, err
	}

	if req.CreateDefaultGroup == nil || *req.CreateDefaultGroup {
		groupReq := &group.CreateGroupRequest{
			Name:   req.Name + " - group",
			TeamID: teamResp.ID,
			IsMain: true,
		}
		if _, err := d.groupSvc.Create(ctx, ownerID, groupReq); err != nil {
			customlogger.Error(ctx, "error creating default group for team", err,
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamResp.ID)),
				customlogger.TagMethod("CreateTeam"))
		}
	}

	return teamResp, nil
}
