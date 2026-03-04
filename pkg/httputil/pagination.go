package httputil

import (
	"net/http"
	"strconv"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Pagination holds parsed limit/offset from query params.
type Pagination struct {
	Limit  int
	Offset int
}

// ParsePagination extracts limit and offset from query params with sane defaults.
func ParsePagination(r *http.Request) Pagination {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > MaxPageSize {
		limit = DefaultPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return Pagination{Limit: limit, Offset: offset}
}
