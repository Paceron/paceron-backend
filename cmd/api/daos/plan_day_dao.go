package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PlanDayDaoInterface interface {
	FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error)
	ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error
}

type planDayDao struct {
	DB *gorm.DB
}

func NewPlanDayDao(database *gorm.DB) PlanDayDaoInterface {
	return &planDayDao{DB: database}
}

func (d *planDayDao) FindByPlan(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
	var days []dbs.PlanDay
	err := d.DB.Where("plan_id = ?", planID).Order("sequence_no").Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("error listing plan days: %w", err)
	}
	return days, nil
}

// ReplaceForPlan borra el set anterior y crea el nuevo en una transacción —
// mismo criterio que SessionExerciseDao.ReplaceForSession.
func (d *planDayDao) ReplaceForPlan(ctx *gin.Context, planID int64, rows []dbs.PlanDay) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&dbs.PlanDay{}).Error; err != nil {
			return fmt.Errorf("error clearing plan days: %w", err)
		}
		for i := range rows {
			rows[i].ID = 0
			rows[i].PlanID = planID
			if err := tx.Create(&rows[i]).Error; err != nil {
				return fmt.Errorf("error creating plan day: %w", err)
			}
		}
		return nil
	})
}
