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
	)
	if err != nil {
		customlogger.Error(nil, "auto-migrate failed", err)
		return nil, err
	}

	customlogger.Info(nil, "DB initialized successfully",
		customlogger.Tag("db_name", configDB.Name))

	return db, nil
}
