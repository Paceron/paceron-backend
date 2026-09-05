package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
)

func TestAssignToDefaultGroup_ExplicitGroupID(t *testing.T) {
	created := false
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			created = true
			assert.Equal(t, int64(42), gu.GroupID)
			return nil
		},
	}

	groupID := int64(42)
	AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)

	assert.True(t, created)
}

func TestAssignToDefaultGroup_FallsBackToMainGroup(t *testing.T) {
	groupDao := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 1, IsMain: false}, {ID: 2, IsMain: true}}, nil
		},
	}
	var createdGroupID int64
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			createdGroupID = gu.GroupID
			return nil
		},
	}

	AssignToDefaultGroup(nil, groupDao, groupUserDao, 1, nil, 7)

	assert.Equal(t, int64(2), createdGroupID)
}

func TestAssignToDefaultGroup_NoDefaultGroup_LogsAndReturns(t *testing.T) {
	groupDao := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 1, IsMain: false}}, nil
		},
	}
	called := false
	groupUserDao := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			called = true
			return nil
		},
	}

	AssignToDefaultGroup(nil, groupDao, groupUserDao, 1, nil, 7)

	assert.False(t, called)
}

func TestAssignToDefaultGroup_AlreadyMember_DoesNotDuplicate(t *testing.T) {
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{GroupID: groupID, UserID: userID}, nil
		},
	}
	called := false
	groupUserDao.createFn = func(ctx *gin.Context, gu *dbs.GroupUser) error {
		called = true
		return nil
	}

	groupID := int64(5)
	AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)

	assert.False(t, called)
}

func TestAssignToDefaultGroup_CreateFails_LogsAndReturns(t *testing.T) {
	groupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			return assert.AnError
		},
	}

	groupID := int64(5)
	assert.NotPanics(t, func() {
		AssignToDefaultGroup(nil, &mockGroupDao{}, groupUserDao, 1, &groupID, 7)
	})
}
