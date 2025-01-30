package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	e "github.com/TaskGroup/dbpsql/app/back/pkg/models/errors"
	"github.com/TaskGroup/dbpsql/app/back/pkg/models/template"
	"log"
	"time"

	_ "github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib" // Это позволяет pgx работать с database/sql
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log/slog"
)

// MaxRetries defines the maximum number of connection retries
const MaxRetries = 50

// RetryInterval defines the interval between retries
const RetryInterval = 5 * time.Second

// Конфигурация пула соединений
const (
	MaxOpenConns    = 5
	MaxIdleConns    = 5
	ConnMaxLifetime = 5 * time.Minute
)

// Queryer определяет интерфейс для выполнения запросов
type Queryer interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
}

// UnitOfWork определяет интерфейс паттерна Unit of Work
type UnitOfWork interface {
	Do(ctx context.Context, fn func(uow UnitOfWork) error) error
	GetQueryer() Queryer
}

// PostgresUnitOfWork реализует интерфейс UnitOfWork
type PostgresUnitOfWork struct {
	db *sqlx.DB
	tx *sqlx.Tx
}

// NewUnitOfWork создает новый экземпляр PostgresUnitOfWork
func NewUnitOfWork(db *sqlx.DB) *PostgresUnitOfWork {
	return &PostgresUnitOfWork{
		db: db,
	}
}

// Do выполняет функцию в рамках транзакции
func (uow *PostgresUnitOfWork) Do(ctx context.Context, fn func(uow UnitOfWork) error) (err error) {
	tx, err := uow.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	uow.tx = tx

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(uow)
	return err
}

// GetQueryer возвращает текущий Queryer (tx или db)
func (uow *PostgresUnitOfWork) GetQueryer() Queryer {
	if uow.tx != nil {
		return uow.tx
	}
	return uow.db
}

// InitDB инициализирует подключение к базе данных и настраивает пул соединений
func InitDB(dsn string) (*sqlx.DB, error) {
	var db *sqlx.DB
	var err error

	for i := 0; i < MaxRetries; i++ {
		db, err = connectDB(dsn)
		if err == nil {
			// Настройка пула соединений
			db.SetMaxOpenConns(MaxOpenConns)
			db.SetMaxIdleConns(MaxIdleConns)
			db.SetConnMaxLifetime(ConnMaxLifetime)

			log.Println("Подключение к базе данных установлено")
			return db, nil
		}

		slog.Error("Не удалось подключиться к базе данных", "попытка", i+1, "ошибка", err)
		time.Sleep(RetryInterval)
	}
	return nil, fmt.Errorf("не удалось подключиться к базе данных после %d попыток: %w", MaxRetries, err)
}

// connectDB устанавливает соединение с базой данных и проверяет его
func connectDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка при открытии соединения: %w", err)
	}

	// Установите таймаут для Ping, чтобы избежать зависания
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ошибка при проверке соединения: %w", err)
	}
	return db, nil
}

// CloseDB закрывает подключение к базе данных
func CloseDB(db *sqlx.DB) error {
	if err := db.Close(); err != nil {
		slog.Error("Ошибка при закрытии соединения с базой данных", "ошибка", err)
		return err
	}
	return nil
}

// QueryMultiple выполняет запрос и сканирует результаты в предоставленный срез
func QueryMultiple(ctx context.Context, queryer Queryer, query string, params map[string]interface{}, dest interface{}) error {
	stmt, err := queryer.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	err = stmt.SelectContext(ctx, dest, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e.ErrNotFound
		}
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	return nil
}

// InsertRecord выполняет вставку новой записи в таблицу и возвращает ID.
func InsertRecord(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) (int64, error) {
	var ids []int64
	err := QueryMultiple(ctx, queryer, query, params, &ids)
	if err != nil {
		return 0, err
	}

	if len(ids) > 0 {
		return ids[0], nil
	}
	return 0, fmt.Errorf("%w: запись не добавлена", e.ErrInternal)
}

// UpdateRecord выполняет обновление записи в таблице.
func UpdateRecord(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) error {
	stmt, err := queryer.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, params)
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	if rowsAffected < 1 {
		return fmt.Errorf("%w: запись не обновлена", e.ErrInternal)
	}
	return nil
}

// UpdateRecordWithResultId выполняет обновление записи в таблице и возвращает ID записи
func UpdateRecordWithResultId(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) (int64, error) {
	return InsertRecord(ctx, queryer, query, params)
}

// UpdateRecordWithResultListId выполняет обновление записей и возвращает их ID
func UpdateRecordWithResultListId(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) ([]template.OnlyId, error) {
	var ids []template.OnlyId
	err := QueryMultiple(ctx, queryer, query, params, &ids)
	if err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		return ids, nil
	}
	return nil, e.ErrNotUpdated
}

// DeleteRecord выполняет удаление записи из таблицы.
func DeleteRecord(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) error {
	stmt, err := queryer.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, params)
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	if rowsAffected < 1 {
		return e.ErrNotFound
	}

	return nil
}

// CheckExistence проверяет, существует ли запись с заданными параметрами.
func CheckExistence(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) (bool, error) {
	var exists bool
	err := QuerySingle(ctx, queryer, query, params, &exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	return exists, nil
}

// CheckExistenceWithError проверяет, существует ли запись с заданными параметрами и возвращает ошибку, если она существует
func CheckExistenceWithError(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) error {
	// Оборачиваем исходный запрос в EXISTS
	existsQuery := fmt.Sprintf(`SELECT EXISTS(%s) AS ex`, query)
	exists, err := CheckExistence(ctx, queryer, existsQuery, params)
	if err != nil {
		return err
	}
	if exists {
		return e.ErrAlreadyExists
	}
	return nil
}

// AddRecord Добавление записи и возврат id добавленной
func AddRecord(ctx context.Context, queryer Queryer, query string, params map[string]interface{}) (int64, error) {
	var res []template.OnlyId
	err := QueryMultiple(ctx, queryer, query, params, &res)
	if err != nil {
		return 0, fmt.Errorf("ошибка добавления: %w", err)
	}
	return res[0].Id, nil
}

// ExecuteNonQuery Обновление/удаление данных (запрос без возврта значений)
func ExecuteNonQuery(qCtx context.Context, queryer Queryer, query string, params map[string]interface{}) error {
	stmt, err := queryer.PrepareNamedContext(qCtx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(qCtx, params)
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	if rowsAffected < 1 {
		return fmt.Errorf("%w: действие не выполнено", e.ErrNotFound)
	}
	return nil
}

func QuerySingle(ctx context.Context, queryer Queryer, query string, params map[string]interface{}, dest interface{}) error {
	stmt, err := queryer.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	err = stmt.GetContext(ctx, dest, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e.ErrNotFound
		}
		return fmt.Errorf("%w: %v", e.ErrInternal, err)
	}
	return nil
}
