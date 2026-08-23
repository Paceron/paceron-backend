package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/pushtoken"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type PushTokenServiceInterface interface {
	RegisterToken(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error
}

type pushTokenService struct {
	pushTokenDao daos.PushTokenDaoInterface
}

func NewPushTokenService(pushTokenDao daos.PushTokenDaoInterface) PushTokenServiceInterface {
	return &pushTokenService{
		pushTokenDao: pushTokenDao,
	}
}

func (s *pushTokenService) RegisterToken(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error {
	if !constants.IsValidPushPlatform(req.Platform) {
		return fmt.Errorf("platform inválida: %s. Valores permitidos: %v", req.Platform, constants.GetValidPushPlatforms())
	}

	if err := s.pushTokenDao.Upsert(ctx, userID, req.Token, req.Platform); err != nil {
		customlogger.Error(ctx, "error registering push token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)))
		return fmt.Errorf("error al registrar el token de push")
	}

	customlogger.Info(ctx, "push token registered successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.TagMethod("RegisterToken"))

	return nil
}
