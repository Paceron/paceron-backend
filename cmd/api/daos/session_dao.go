package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type SessionDaoInterface interface {
	Create(ctx *gin.Context, s *dbs.Session) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Session, error)
	// FindByIDs trae varias sesiones de catálogo en una sola query
	// (instanciación batch del write path del calendario, design.md D6).
	FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Session, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error)
	Update(ctx *gin.Context, s *dbs.Session) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type sessionDao struct {
	DB *gorm.DB
}

func NewSessionDao(database *gorm.DB) SessionDaoInterface {
	return &sessionDao{DB: database}
}

func (d *sessionDao) Create(ctx *gin.Context, s *dbs.Session) error {
	return d.DB.Create(s).Error
}

func (d *sessionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Session, error) {
	var s dbs.Session
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding session: %w", err)
	}
	return &s, nil
}

// FindByIDs aspira igual que FindByID (sin borrados). Duplicados en ids
// devuelven una fila por id distinto.
func (d *sessionDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Session, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var sessions []dbs.Session
	err := d.DB.Where("id IN ? AND deleted_at IS NULL", ids).Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error finding sessions by ids: %w", err)
	}
	return sessions, nil
}

func (d *sessionDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.Session, error) {
	var sessions []dbs.Session
	err := d.DB.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("id").Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("error listing sessions: %w", err)
	}
	return sessions, nil
}

func (d *sessionDao) Update(ctx *gin.Context, s *dbs.Session) error {
	return d.DB.Model(&dbs.Session{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
		"name":        s.Name,
		"description": s.Description,
	}).Error
}

func (d *sessionDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Session{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}
