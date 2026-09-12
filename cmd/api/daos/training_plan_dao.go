package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type TrainingPlanDaoInterface interface {
	Create(ctx *gin.Context, p *dbs.TrainingPlan) error
	FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error)
	FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error)
	Update(ctx *gin.Context, p *dbs.TrainingPlan) error
	Delete(ctx *gin.Context, id int64) error
}

type trainingPlanDao struct {
	DB *gorm.DB
}

func NewTrainingPlanDao(database *gorm.DB) TrainingPlanDaoInterface {
	return &trainingPlanDao{DB: database}
}

func (d *trainingPlanDao) Create(ctx *gin.Context, p *dbs.TrainingPlan) error {
	return d.DB.Create(p).Error
}

func (d *trainingPlanDao) FindByID(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
	var p dbs.TrainingPlan
	err := d.DB.Where("id = ?", id).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding training plan: %w", err)
	}
	return &p, nil
}

func (d *trainingPlanDao) FindByOwner(ctx *gin.Context, ownerID int64) ([]dbs.TrainingPlan, error) {
	var plans []dbs.TrainingPlan
	err := d.DB.Where("owner_id = ?", ownerID).Order("id").Find(&plans).Error
	if err != nil {
		return nil, fmt.Errorf("error listing training plans: %w", err)
	}
	return plans, nil
}

func (d *trainingPlanDao) Update(ctx *gin.Context, p *dbs.TrainingPlan) error {
	return d.DB.Model(&dbs.TrainingPlan{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"name":        p.Name,
		"description": p.Description,
	}).Error
}

// Delete es físico — sin caducidad ni soft-delete para TrainingPlan (D1).
// Borra sus PlanDay en la misma transacción (sin FK física en este repo).
func (d *trainingPlanDao) Delete(ctx *gin.Context, id int64) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", id).Delete(&dbs.PlanDay{}).Error; err != nil {
			return fmt.Errorf("error deleting plan days: %w", err)
		}
		if err := tx.Delete(&dbs.TrainingPlan{}, id).Error; err != nil {
			return fmt.Errorf("error deleting training plan: %w", err)
		}
		return nil
	})
}
