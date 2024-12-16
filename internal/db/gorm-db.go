package db

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectGORM(databaseName string) (*gorm.DB, error) {
	dabaseUrl := os.Getenv(fmt.Sprintf("database_gorm_url_%s", databaseName))
	if dabaseUrl == `` {
		return nil, fmt.Errorf("not found database_gorm_url")
	}

	db, err := gorm.Open(postgres.Open(dabaseUrl), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("not connect gorm")
	}

	return db, nil
}

func CloseGORM(gormDB *gorm.DB) error {
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sqlDB from GORM: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		fmt.Printf("Failed to close sqlDB: %v", err)
	}

	return nil
}
