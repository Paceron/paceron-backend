package postgresdb

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

func ConfigDB(configDB config.DB) (*gorm.DB, error) {
	loc := time.UTC

	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
		configDB.Host,
		configDB.Port,
		configDB.Username,
		configDB.Password,
		configDB.Name,
		loc.String(),
	)

	db, err := gorm.Open(postgres.Open(connString), &gorm.Config{
		Logger: gl.Default.LogMode(gl.Silent),
	})

	if err != nil {
		customlogger.Error(nil, "cannot open postgres DB", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		customlogger.Error(nil, "cannot get sql.DB instance", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	sqlDB.SetConnMaxLifetime(configDB.ConnMaxLifetime)
	sqlDB.SetMaxIdleConns(configDB.MaxIdleConnections)
	sqlDB.SetMaxOpenConns(configDB.MaxOpenConnections)

	err = sqlDB.Ping()
	if err != nil {
		customlogger.Error(nil, "error connecting to DB", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	customlogger.Info(nil, "DB connected successfully",
		customlogger.Tag("db_name", configDB.Name),
		customlogger.Tag("host", configDB.Host),
		customlogger.Tag("port", configDB.Port))

	stats := sqlDB.Stats()
	if stats.OpenConnections >= configDB.MaxOpenConnections {
		return nil, fmt.Errorf(
			"[DBNAME:%s] number of connections exceeded: %v",
			configDB.Name,
			stats.OpenConnections,
		)
	}

	err = db.AutoMigrate(
		&dbs.User{},
		&dbs.Permission{},
		&dbs.Role{},
		&dbs.Tier{},
		&dbs.TierPermission{},
		&dbs.UserRole{},
		&dbs.PasswordResetToken{},
		&dbs.Team{},
		&dbs.Group{},
		&dbs.TeamUser{},
		&dbs.GroupUser{},
		&dbs.Invitation{},
		&dbs.RefreshToken{},
		&dbs.PushToken{},
		&dbs.Payment{},
		&dbs.UserRoleTierSubscription{},
		&dbs.Installment{},
	)
	if err != nil {
		customlogger.Error(nil, "auto-migrate failed", err)
		return nil, err
	}

	// Constraints que GORM no expresa por tags: se crean con SQL crudo post-
	// AutoMigrate, idempotente (vuelve a correr sin error si ya existen).
	// 1. Una sola suscripción vigente por (user_id, role_id): índice único parcial.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_sub_ids_user_role_active
		ON user_role_tier_subscriptions (user_id, role_id)
		WHERE status IN ('active','first_payment_pending');`).Error; err != nil {
		customlogger.Error(nil, "error creating partial unique index on user_role_tier_subscriptions", err)
		return nil, err
	}

	// 2. Arco exclusivo en installments: exactamente uno de subscription_id o team_id.
	if err := db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_installments_exclusive_parent') THEN
			ALTER TABLE installments ADD CONSTRAINT chk_installments_exclusive_parent
			CHECK (num_nonnulls(subscription_id, team_id) = 1);
		END IF;
	END $$;`).Error; err != nil {
		customlogger.Error(nil, "error creating exclusive parent check on installments", err)
		return nil, err
	}

	// 3. FK de payments.installment_id -> installments.id (columna aditiva sobre
	// tabla existente, no rompe nada; reintentable porque está en un DO guardado).
	if err := db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_payments_installment') THEN
			ALTER TABLE payments ADD CONSTRAINT fk_payments_installment
			FOREIGN KEY (installment_id) REFERENCES installments(id) ON DELETE SET NULL;
		END IF;
	END $$;`).Error; err != nil {
		customlogger.Error(nil, "error creating payments installment FK", err)
		return nil, err
	}

	customlogger.Info(nil, "DB initialized successfully",
		customlogger.Tag("db_name", configDB.Name))

	return db, nil
}
