package main

import (
	"context"
	"fmt"
	"github.com/TaskGroup/dbpsql/app/back/config"
	"github.com/TaskGroup/dbpsql/app/back/pkg/migration/goose"
	e "github.com/TaskGroup/dbpsql/app/back/pkg/models/errors"
	"github.com/TaskGroup/dbpsql/app/back/pkg/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type TestTable struct {
	Id   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

func main() {
	ctx := context.Background()
	cfg := config.MustLoad()
	var err error

	if err = goose.InitMigrations(cfg.DBPostgres.DSN, cfg.DBPostgres.MigrationsPath); err != nil {
		panic(errors.New("init migrations failed: " + err.Error()))
	}
	DB, err := postgres.InitDB(cfg.DBPostgres.DSN)
	if err != nil {
		panic(errors.New("init db failed: " + err.Error()))
	}

	defer postgres.CloseDB(DB)
	all, err := getAll(ctx, DB)
	if err != nil {
		return
	}
	fmt.Println("all:", all)

	one, err := getOne(ctx, DB, 2)
	if err != nil {
		return
	}
	fmt.Println("one:", one)

	if err = existsTestTable(ctx, DB, 3); err != nil {
		if errors.Is(err, e.ErrAlreadyExists) {
			fmt.Println("Record already exists")
		} else if err == nil {
			fmt.Println("Record not exists")
		} else {
			fmt.Println("Возникла ошибка при выполнении запроса: %w", err)
		}
	}
	fmt.Println("stopped example")
}

// Список всех
func getAll(ctx context.Context, db *sqlx.DB) ([]TestTable, error) {
	var res []TestTable
	query := `select  t.id, t.name 
 				from public."test_table" t`
	if err := postgres.QueryMultiple(ctx, db, query, map[string]interface{}{}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// Один элемент
func getOne(ctx context.Context, db *sqlx.DB, id int64) (*TestTable, error) {
	var res TestTable
	query := ` select  t.id, t.name  
 				from public."test_table" t
 				where t.id =:idTestTable`
	if err := postgres.QuerySingle(ctx, db, query, map[string]interface{}{
		"idTestTable": id,
	}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Существование элемента
func existsTestTable(ctx context.Context, db *sqlx.DB, id int64) error {
	query := `select  1
 				from public."test_table" t
 				where t.id =:idTestTable`
	return postgres.CheckExistenceWithError(ctx, db, query, map[string]interface{}{
		"idTestTable": id,
	})
}
