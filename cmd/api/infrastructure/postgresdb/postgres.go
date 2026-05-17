package postgresdb

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/config"

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
		fmt.Printf("Cannot open postgres DB [%s]: %v\n", configDB.Name, err)
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		fmt.Printf("Cannot get sql.DB instance [%s]: %v\n", configDB.Name, err)
		return nil, err
	}

	sqlDB.SetConnMaxLifetime(configDB.ConnMaxLifetime)
	sqlDB.SetMaxIdleConns(configDB.MaxIdleConnections)
	sqlDB.SetMaxOpenConns(configDB.MaxOpenConnections)

	err = sqlDB.Ping()
	if err != nil {
		fmt.Printf("Error connecting to DB [%s]: %v\n", configDB.Name, err)
		return nil, err
	}

	stats := sqlDB.Stats()
	if stats.OpenConnections >= configDB.MaxOpenConnections {
		return nil, fmt.Errorf(
			"[DBNAME:%s] number of connections exceeded: %v",
			configDB.Name,
			stats.OpenConnections,
		)
	}

	fmt.Printf("INIT DB SUCCESS [%s]\n", configDB.Name)

	return db, nil
}
