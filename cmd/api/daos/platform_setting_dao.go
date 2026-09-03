package daos

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// PlatformSettingDaoInterface define el acceso a la tabla genérica key-value de
// configuración global de la plataforma.
type PlatformSettingDaoInterface interface {
	Get(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error)
	Set(ctx *gin.Context, key string, value float64, updatedBy int64) (*dbs.PlatformSetting, error)
}

type platformSettingDao struct {
	DB *gorm.DB
}

func NewPlatformSettingDao(database *gorm.DB) PlatformSettingDaoInterface {
	return &platformSettingDao{DB: database}
}

// Get devuelve el valor numérico de una key. Si no existe, devuelve defaultValue
// (SETTING: el registro no se crea hasta el primer Set — comportamiento determinístico).
func (d *platformSettingDao) Get(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error) {
	var setting dbs.PlatformSetting
	err := d.DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return defaultValue, nil, nil
		}
		return 0, nil, fmt.Errorf("error finding platform setting: %w", err)
	}
	var value float64
	if err := json.Unmarshal([]byte(setting.Value), &value); err != nil {
		return 0, nil, fmt.Errorf("error parsing platform setting value: %w", err)
	}
	return value, &setting, nil
}

// Set persiste (upsert) el valor numérico de una key como JSON, registrando updated_by.
func (d *platformSettingDao) Set(ctx *gin.Context, key string, value float64, updatedBy int64) (*dbs.PlatformSetting, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("error marshaling platform setting value: %w", err)
	}

	var setting dbs.PlatformSetting
	err = d.DB.Where("key = ?", key).First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		setting = dbs.PlatformSetting{
			Key:       key,
			Value:     string(raw),
			UpdatedBy: &updatedBy,
		}
		if err := d.DB.Create(&setting).Error; err != nil {
			return nil, fmt.Errorf("error creating platform setting: %w", err)
		}
		return &setting, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error finding platform setting: %w", err)
	}

	setting.Value = string(raw)
	setting.UpdatedBy = &updatedBy
	if err := d.DB.Save(&setting).Error; err != nil {
		return nil, fmt.Errorf("error updating platform setting: %w", err)
	}
	return &setting, nil
}
