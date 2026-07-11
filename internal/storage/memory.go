package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jiangchengyu998/demo-go/internal/item"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]item.Item
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID: 1,
		items:  make(map[int64]item.Item),
	}
}

func (r *MemoryRepository) List(_ context.Context, page, size int, sortSpec string) ([]item.Item, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]item.Item, 0, len(r.items))
	for _, current := range r.items {
		items = append(items, current)
	}
	sortItems(items, sortSpec)

	offset := page * size
	if offset >= len(items) {
		return []item.Item{}, int64(len(items)), nil
	}
	end := min(offset+size, len(items))
	return append([]item.Item(nil), items[offset:end]...), int64(len(items)), nil
}

func (r *MemoryRepository) Get(_ context.Context, id int64) (item.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	current, ok := r.items[id]
	if !ok {
		return item.Item{}, item.NotFoundError{ID: id}
	}
	return current, nil
}

func (r *MemoryRepository) Create(_ context.Context, request item.Request) (item.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	current := item.Item{
		ID:          r.nextID,
		Name:        request.Name,
		Description: request.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.items[current.ID] = current
	r.nextID++
	return current, nil
}

func (r *MemoryRepository) Update(_ context.Context, id int64, request item.Request) (item.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.items[id]
	if !ok {
		return item.Item{}, item.NotFoundError{ID: id}
	}
	current.Name = request.Name
	current.Description = request.Description
	current.UpdatedAt = time.Now().UTC()
	r.items[id] = current
	return current, nil
}

func (r *MemoryRepository) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return item.NotFoundError{ID: id}
	}
	delete(r.items, id)
	return nil
}

func sortItems(items []item.Item, sortSpec string) {
	column, descending := parseSort(sortSpec)
	sort.SliceStable(items, func(i, j int) bool {
		var less bool
		switch column {
		case "name":
			less = items[i].Name < items[j].Name
		case "createdAt", "created_at":
			less = items[i].CreatedAt.Before(items[j].CreatedAt)
		case "updatedAt", "updated_at":
			less = items[i].UpdatedAt.Before(items[j].UpdatedAt)
		default:
			less = items[i].ID < items[j].ID
		}
		if descending {
			return !less
		}
		return less
	})
}

func parseSort(sortSpec string) (string, bool) {
	parts := strings.Split(sortSpec, ",")
	column := strings.TrimSpace(parts[0])
	descending := false
	if strings.HasPrefix(column, "-") {
		column = strings.TrimPrefix(column, "-")
		descending = true
	}
	if len(parts) > 1 && strings.EqualFold(strings.TrimSpace(parts[1]), "desc") {
		descending = true
	}
	if column == "" {
		column = "id"
	}
	return column, descending
}
