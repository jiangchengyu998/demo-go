package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

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
	db           *sql.DB
	databaseName string
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
	return &MySQLRepository{db: db, databaseName: settings.DatabaseName}, nil
}

func (r *MySQLRepository) Close() error {
	return r.db.Close()
}

func (r *MySQLRepository) List(ctx context.Context, page, size int, sortSpec string) ([]item.Item, int64, error) {
	var total int64
	countCtx, countSpan := r.startDBSpan(ctx, "SELECT items count", "SELECT", "items", "SELECT COUNT(*) FROM items")
	if err := r.db.QueryRowContext(countCtx, "SELECT COUNT(*) FROM items").Scan(&total); err != nil {
		recordSpanError(countSpan, err)
		countSpan.End()
		return nil, 0, fmt.Errorf("count items: %w", err)
	}
	countSpan.End()

	orderBy := orderByClause(sortSpec)
	listCtx, listSpan := r.startDBSpan(ctx, "SELECT items", "SELECT", "items", "SELECT id, name, description, created_at, updated_at FROM items ORDER BY ? LIMIT ? OFFSET ?")
	rows, err := r.db.QueryContext(listCtx,
		"SELECT id, name, description, created_at, updated_at FROM items ORDER BY "+orderBy+" LIMIT ? OFFSET ?",
		size,
		page*size,
	)
	if err != nil {
		recordSpanError(listSpan, err)
		listSpan.End()
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	defer listSpan.End()

	items := make([]item.Item, 0, size)
	for rows.Next() {
		current, err := scanItem(rows)
		if err != nil {
			recordSpanError(listSpan, err)
			return nil, 0, err
		}
		items = append(items, current)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(listSpan, err)
		return nil, 0, fmt.Errorf("iterate items: %w", err)
	}
	return items, total, nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (item.Item, error) {
	ctx, span := r.startDBSpan(ctx, "SELECT item", "SELECT", "items", "SELECT id, name, description, created_at, updated_at FROM items WHERE id = ?")
	defer span.End()

	row := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at, updated_at FROM items WHERE id = ?", id)
	current, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return item.Item{}, item.NotFoundError{ID: id}
	}
	if err != nil {
		recordSpanError(span, err)
		return item.Item{}, err
	}
	return current, nil
}

func (r *MySQLRepository) Create(ctx context.Context, request item.Request) (item.Item, error) {
	dbCtx, span := r.startDBSpan(ctx, "INSERT item", "INSERT", "items", "INSERT INTO items (name, description) VALUES (?, ?)")
	result, err := r.db.ExecContext(dbCtx, "INSERT INTO items (name, description) VALUES (?, ?)", request.Name, nullableString(request.Description))
	if err != nil {
		recordSpanError(span, err)
		span.End()
		return item.Item{}, fmt.Errorf("create item: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		recordSpanError(span, err)
		span.End()
		return item.Item{}, fmt.Errorf("read created item id: %w", err)
	}
	span.End()
	return r.Get(ctx, id)
}

func (r *MySQLRepository) Update(ctx context.Context, id int64, request item.Request) (item.Item, error) {
	dbCtx, span := r.startDBSpan(ctx, "UPDATE item", "UPDATE", "items", "UPDATE items SET name = ?, description = ? WHERE id = ?")
	result, err := r.db.ExecContext(dbCtx, "UPDATE items SET name = ?, description = ? WHERE id = ?", request.Name, nullableString(request.Description), id)
	if err != nil {
		recordSpanError(span, err)
		span.End()
		return item.Item{}, fmt.Errorf("update item: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		span.End()
		return item.Item{}, fmt.Errorf("read update result: %w", err)
	}
	if affected == 0 {
		span.End()
		return item.Item{}, item.NotFoundError{ID: id}
	}
	span.End()
	return r.Get(ctx, id)
}

func (r *MySQLRepository) Delete(ctx context.Context, id int64) error {
	ctx, span := r.startDBSpan(ctx, "DELETE item", "DELETE", "items", "DELETE FROM items WHERE id = ?")
	defer span.End()

	result, err := r.db.ExecContext(ctx, "DELETE FROM items WHERE id = ?", id)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("delete item: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("read delete result: %w", err)
	}
	if affected == 0 {
		return item.NotFoundError{ID: id}
	}
	return nil
}

func (r *MySQLRepository) startDBSpan(ctx context.Context, name, operation, collection, statement string) (context.Context, trace.Span) {
	databaseName := r.databaseName
	if databaseName == "" {
		databaseName = "cloud_deploy_demo"
	}
	return otel.Tracer("cloud-deploy-demo-go/mysql").Start(ctx, "mysql "+name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "mysql"),
			attribute.String("db.name", databaseName),
			attribute.String("db.operation", operation),
			attribute.String("db.sql.table", collection),
			attribute.String("db.namespace", databaseName),
			attribute.String("db.statement", statement),
		),
	)
}

func recordSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
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
