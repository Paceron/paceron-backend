package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// AssignToDefaultGroup da de alta a userID en groupID, o en el grupo principal
// (IsMain) de teamID si groupID es nil. Best-effort: nunca devuelve error ni
// bloquea al caller, solo loguea si falla — extraído de invitation_service.go
// (antes assignInviteeToGroup, atado a *dbs.Invitation) para que join_request_service
// use la misma lógica sin duplicarla.
func AssignToDefaultGroup(
	ctx *gin.Context,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	teamID int64,
	groupID *int64,
	userID int64,
) {
	targetGroupID := groupID

	if targetGroupID == nil {
		groups, err := groupDao.GetByTeamID(ctx, teamID)
		if err != nil {
			customlogger.Error(ctx, "error finding team groups for default group assignment", err,
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
				customlogger.TagMethod("AssignToDefaultGroup"))
			return
		}
		for _, g := range groups {
			if g.IsMain {
				id := g.ID
				targetGroupID = &id
				break
			}
		}
		if targetGroupID == nil {
			customlogger.Warn(ctx, "no default group found for team on membership assignment",
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
				customlogger.TagMethod("AssignToDefaultGroup"))
			return
		}
	}

	existingGroupMember, err := groupUserDao.FindByGroupAndUser(ctx, *targetGroupID, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking group membership on membership assignment", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AssignToDefaultGroup"))
		return
	}
	if existingGroupMember != nil {
		return
	}

	groupUser := &dbs.GroupUser{
		GroupID:   *targetGroupID,
		UserID:    userID,
		DateStart: time.Now(),
	}
	if err := groupUserDao.Create(ctx, groupUser); err != nil {
		customlogger.Error(ctx, "error creating group_user on membership assignment", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("group_id", fmt.Sprintf("%d", *targetGroupID)),
			customlogger.TagMethod("AssignToDefaultGroup"))
	}
}
