package item

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ErrNotFound   = errors.New("item not found")
	ErrValidation = errors.New("validation failed")
)

type Item struct {
	ID          int64
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Page struct {
	Content       []Item
	Page          int
	Size          int
	TotalElements int64
	TotalPages    int
	First         bool
	Last          bool
}

type Request struct {
	Name        string
	Description *string
}

type Repository interface {
	List(ctx context.Context, page, size int, sort string) ([]Item, int64, error)
	Get(ctx context.Context, id int64) (Item, error)
	Create(ctx context.Context, request Request) (Item, error)
	Update(ctx context.Context, id int64, request Request) (Item, error)
	Delete(ctx context.Context, id int64) error
}

type Service struct {
	repository Repository
	logger     *slog.Logger
}

func NewService(repository Repository, logger *slog.Logger) *Service {
	return &Service{repository: repository, logger: logger}
}

func (s *Service) List(ctx context.Context, page, size int, sort string) (Page, error) {
	page = max(page, 0)
	size = min(max(size, 1), 100)
	if strings.TrimSpace(sort) == "" {
		sort = "id"
	}

	ctx, span := otel.Tracer("cloud-deploy-demo-go/item").Start(ctx, "ItemService.list")
	defer span.End()
	span.SetAttributes(attribute.Int("page", page), attribute.Int("size", size))

	items, total, err := s.repository.List(ctx, page, size, sort)
	if err != nil {
		span.RecordError(err)
		return Page{}, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}

	response := Page{
		Content:       items,
		Page:          page,
		Size:          size,
		TotalElements: total,
		TotalPages:    totalPages,
		First:         page == 0,
		Last:          totalPages == 0 || page >= totalPages-1,
	}
	s.logger.InfoContext(ctx, "ItemService.list", "page", page, "size", size, "totalElements", total)
	return response, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Item, error) {
	ctx, span := otel.Tracer("cloud-deploy-demo-go/item").Start(ctx, "ItemService.get")
	defer span.End()
	span.SetAttributes(attribute.Int64("item.id", id))

	item, err := s.repository.Get(ctx, id)
	if err != nil {
		span.RecordError(err)
		return Item{}, err
	}
	s.logger.InfoContext(ctx, "ItemService.get", "id", id)
	return item, nil
}

func (s *Service) Create(ctx context.Context, request Request) (Item, error) {
	if err := validateRequest(request); err != nil {
		return Item{}, err
	}

	ctx, span := otel.Tracer("cloud-deploy-demo-go/item").Start(ctx, "ItemService.create")
	defer span.End()
	span.SetAttributes(attribute.String("item.name", request.Name))

	item, err := s.repository.Create(ctx, request)
	if err != nil {
		span.RecordError(err)
		return Item{}, err
	}
	s.logger.InfoContext(ctx, "ItemService.create", "id", item.ID, "name", item.Name)
	return item, nil
}

func (s *Service) Update(ctx context.Context, id int64, request Request) (Item, error) {
	if err := validateRequest(request); err != nil {
		return Item{}, err
	}

	ctx, span := otel.Tracer("cloud-deploy-demo-go/item").Start(ctx, "ItemService.update")
	defer span.End()
	span.SetAttributes(attribute.Int64("item.id", id), attribute.String("item.name", request.Name))

	item, err := s.repository.Update(ctx, id, request)
	if err != nil {
		span.RecordError(err)
		return Item{}, err
	}
	s.logger.InfoContext(ctx, "ItemService.update", "id", item.ID, "name", item.Name)
	return item, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	ctx, span := otel.Tracer("cloud-deploy-demo-go/item").Start(ctx, "ItemService.delete")
	defer span.End()
	span.SetAttributes(attribute.Int64("item.id", id))

	if err := s.repository.Delete(ctx, id); err != nil {
		span.RecordError(err)
		return err
	}
	s.logger.InfoContext(ctx, "ItemService.delete", "id", id)
	return nil
}

func validateRequest(request Request) error {
	var details []string
	if strings.TrimSpace(request.Name) == "" {
		details = append(details, "name: must not be blank")
	}
	if utf8.RuneCountInString(request.Name) > 80 {
		details = append(details, "name: must be at most 80 characters")
	}
	if request.Description != nil && utf8.RuneCountInString(*request.Description) > 500 {
		details = append(details, "description: must be at most 500 characters")
	}
	if len(details) > 0 {
		return ValidationError{Details: details}
	}
	return nil
}

type ValidationError struct {
	Details []string
}

type NotFoundError struct {
	ID int64
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Item not found: %d", e.ID)
}

func (e NotFoundError) Unwrap() error {
	return ErrNotFound
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%v: %s", ErrValidation, strings.Join(e.Details, "; "))
}

func (e ValidationError) Unwrap() error {
	return ErrValidation
}
