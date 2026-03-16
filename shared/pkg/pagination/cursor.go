package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// CursorPage holds cursor-based pagination parameters.
type CursorPage struct {
	Cursor string
	Limit  int
}

// OffsetPage holds offset-based pagination parameters.
type OffsetPage struct {
	Page  int
	Limit int
}

// CursorResult is a generic paginated result with a next cursor.
type CursorResult[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// OffsetResult is a generic paginated result with total count.
type OffsetResult[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}

// ParseCursor extracts cursor pagination params from a Gin context.
func ParseCursor(c *gin.Context) CursorPage {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return CursorPage{
		Cursor: c.Query("cursor"),
		Limit:  limit,
	}
}

// ParseOffset extracts offset pagination params from a Gin context.
func ParseOffset(c *gin.Context) OffsetPage {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return OffsetPage{Page: page, Limit: limit}
}

// Offset calculates the SQL OFFSET for a given OffsetPage.
func (p OffsetPage) Offset() int {
	return (p.Page - 1) * p.Limit
}

// TotalPages calculates the number of pages from a total count.
func (p OffsetPage) TotalPages(total int64) int {
	if p.Limit == 0 {
		return 0
	}
	pages := int(total) / p.Limit
	if int(total)%p.Limit > 0 {
		pages++
	}
	return pages
}

// EncodeCursor base64-encodes a cursor payload (any JSON-serializable value).
func EncodeCursor(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// DecodeCursor base64-decodes a cursor into the target struct.
func DecodeCursor(cursor string, target interface{}) error {
	if cursor == "" {
		return errors.New("empty cursor")
	}
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

// NewOffsetResult constructs an OffsetResult.
func NewOffsetResult[T any](data []T, total int64, page OffsetPage) OffsetResult[T] {
	return OffsetResult[T]{
		Data:       data,
		Total:      total,
		Page:       page.Page,
		Limit:      page.Limit,
		TotalPages: page.TotalPages(total),
	}
}

// NewCursorResult constructs a CursorResult.
func NewCursorResult[T any](data []T, nextCursor string) CursorResult[T] {
	return CursorResult[T]{
		Data:       data,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}
}
