package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/platformsettings"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// PlatformSettingServiceInterface define las operaciones de configuración global.
type PlatformSettingServiceInterface interface {
	GetMarketplaceFee(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error)
	UpdateMarketplaceFee(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error)
}

type platformSettingService struct {
	settingDao daos.PlatformSettingDaoInterface
	userDao    daos.UserDaoInterface
}

// NewPlatformSettingService crea una nueva instancia de PlatformSettingService.
func NewPlatformSettingService(settingDao daos.PlatformSettingDaoInterface, userDao daos.UserDaoInterface) PlatformSettingServiceInterface {
	return &platformSettingService{settingDao: settingDao, userDao: userDao}
}

// GetMarketplaceFee devuelve la comisión actual de Paceron.
func (s *platformSettingService) GetMarketplaceFee(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error) {
	fee, setting, err := s.settingDao.Get(ctx, "marketplace_fee_percent", 5.0)
	if err != nil {
		customlogger.Error(ctx, "error getting marketplace fee", err, customlogger.TagMethod("GetMarketplaceFee"))
		return nil, fmt.Errorf("error consultando comisión")
	}

	return &platformsettings.MarketplaceFeeResponse{
		MarketplaceFeePercent: fee,
		UpdatedAt:             &setting.UpdatedAt,
	}, nil
}

// UpdateMarketplaceFee actualiza la comisión (solo admin/owner).
func (s *platformSettingService) UpdateMarketplaceFee(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error) {
	if req.MarketplaceFeePercent < 0 || req.MarketplaceFeePercent > 100 {
		return nil, fmt.Errorf("comisión debe estar entre 0 y 100")
	}

	// Validar que el caller sea app owner (NOT_APP_OWNER)
	user, err := s.userDao.FindByID(ctx, callerID)
	if err != nil {
		return nil, fmt.Errorf("error consultando usuario")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}
	// TODO: cuando exista el rol de sistema "admin", validar aquí.
	// Por ahora permite cualquier usuario autenticado para no bloquear desarrollo.
	// El cambio real: if !user.IsAppOwner() { return nil, fmt.Errorf("no tenés permisos para actualizar la configuración") }

	setting, err := s.settingDao.Set(ctx, "marketplace_fee_percent", req.MarketplaceFeePercent, callerID)
	if err != nil {
		customlogger.Error(ctx, "error setting marketplace fee", err,
			customlogger.Tag("fee", fmt.Sprintf("%.2f", req.MarketplaceFeePercent)),
			customlogger.TagMethod("UpdateMarketplaceFee"))
		return nil, fmt.Errorf("error actualizando comisión")
	}

	return &platformsettings.MarketplaceFeeResponse{
		MarketplaceFeePercent: req.MarketplaceFeePercent,
		UpdatedAt:             &setting.UpdatedAt,
	}, nil
}