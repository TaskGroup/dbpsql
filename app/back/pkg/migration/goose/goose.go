package goose

import (
	"database/sql"
	"fmt"
	"github.com/pressly/goose/v3"
)

func InitMigrations(dsn, dirPath string) error {
	db, err := sql.Open("postgres", dsn)

	if err != nil {
		return fmt.Errorf("error InitMigrations : %w", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return fmt.Errorf("error InitMigrations ping: %w", err)
	}
	err = goose.Up(db, dirPath)
	if err != nil {
		return fmt.Errorf("error up migrations %w: ", err)
	}
	return nil
}
