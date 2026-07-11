package httpapi

import (
	"time"

	"github.com/jiangchengyu998/demo-go/internal/item"
)

type itemRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type itemResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type pageResponse struct {
	Content       []itemResponse `json:"content"`
	Page          int            `json:"page"`
	Size          int            `json:"size"`
	TotalElements int64          `json:"totalElements"`
	TotalPages    int            `json:"totalPages"`
	First         bool           `json:"first"`
	Last          bool           `json:"last"`
}

type apiError struct {
	Timestamp string   `json:"timestamp"`
	Status    int      `json:"status"`
	Error     string   `json:"error"`
	Message   string   `json:"message"`
	Details   []string `json:"details"`
}

func itemResponseFromItem(source item.Item) itemResponse {
	return itemResponse{
		ID:          source.ID,
		Name:        source.Name,
		Description: source.Description,
		CreatedAt:   formatTime(source.CreatedAt),
		UpdatedAt:   formatTime(source.UpdatedAt),
	}
}

func pageResponseFromItemPage(source item.Page) pageResponse {
	content := make([]itemResponse, 0, len(source.Content))
	for _, current := range source.Content {
		content = append(content, itemResponseFromItem(current))
	}
	return pageResponse{
		Content:       content,
		Page:          source.Page,
		Size:          source.Size,
		TotalElements: source.TotalElements,
		TotalPages:    source.TotalPages,
		First:         source.First,
		Last:          source.Last,
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
