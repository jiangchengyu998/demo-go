package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jiangchengyu998/demo-go/internal/config"
	"github.com/jiangchengyu998/demo-go/internal/item"
)

const createItemsSQL = `CREATE TABLE IF NOT EXISTS items (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(80) NOT NULL,
    description VARCHAR(500) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(ctx context.Context, settings config.DatabaseSettings) (*MySQLRepository, error) {
	if settings.ServerDSN != "" && settings.DatabaseName != "" {
		if err := ensureDatabase(ctx, settings); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("mysql", settings.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if _, err := db.ExecContext(ctx, createItemsSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate mysql: %w", err)
	}
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) Close() error {
	return r.db.Close()
}

func (r *MySQLRepository) List(ctx context.Context, page, size int, sortSpec string) ([]item.Item, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count items: %w", err)
	}

	orderBy := orderByClause(sortSpec)
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, description, created_at, updated_at FROM items ORDER BY "+orderBy+" LIMIT ? OFFSET ?",
		size,
		page*size,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	items := make([]item.Item, 0, size)
	for rows.Next() {
		current, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, current)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate items: %w", err)
	}
	return items, total, nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (item.Item, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at, updated_at FROM items WHERE id = ?", id)
	current, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return item.Item{}, item.NotFoundError{ID: id}
	}
	if err != nil {
		return item.Item{}, err
	}
	return current, nil
}

func (r *MySQLRepository) Create(ctx context.Context, request item.Request) (item.Item, error) {
	result, err := r.db.ExecContext(ctx, "INSERT INTO items (name, description) VALUES (?, ?)", request.Name, nullableString(request.Description))
	if err != nil {
		return item.Item{}, fmt.Errorf("create item: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return item.Item{}, fmt.Errorf("read created item id: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *MySQLRepository) Update(ctx context.Context, id int64, request item.Request) (item.Item, error) {
	result, err := r.db.ExecContext(ctx, "UPDATE items SET name = ?, description = ? WHERE id = ?", request.Name, nullableString(request.Description), id)
	if err != nil {
		return item.Item{}, fmt.Errorf("update item: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return item.Item{}, fmt.Errorf("read update result: %w", err)
	}
	if affected == 0 {
		return item.Item{}, item.NotFoundError{ID: id}
	}
	return r.Get(ctx, id)
}

func (r *MySQLRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result: %w", err)
	}
	if affected == 0 {
		return item.NotFoundError{ID: id}
	}
	return nil
}

type itemScanner interface {
	Scan(dest ...any) error
}

func scanItem(scanner itemScanner) (item.Item, error) {
	var current item.Item
	var description sql.NullString
	if err := scanner.Scan(&current.ID, &current.Name, &description, &current.CreatedAt, &current.UpdatedAt); err != nil {
		return item.Item{}, err
	}
	if description.Valid {
		current.Description = &description.String
	}
	return current, nil
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func ensureDatabase(ctx context.Context, settings config.DatabaseSettings) error {
	db, err := sql.Open("mysql", settings.ServerDSN)
	if err != nil {
		return fmt.Errorf("open mysql server: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql server: %w", err)
	}
	statement := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", strings.ReplaceAll(settings.DatabaseName, "`", "``"))
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

func orderByClause(sortSpec string) string {
	column, descending := parseSort(sortSpec)
	sqlColumn := map[string]string{
		"id":         "id",
		"name":       "name",
		"createdAt":  "created_at",
		"created_at": "created_at",
		"updatedAt":  "updated_at",
		"updated_at": "updated_at",
	}[column]
	if sqlColumn == "" {
		sqlColumn = "id"
	}
	if descending {
		return sqlColumn + " DESC"
	}
	return sqlColumn + " ASC"
}
