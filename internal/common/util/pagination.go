package util

import "math"

const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
)

// NormalizePagination returns safe page and limit values.
func NormalizePagination(page, limit int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return page, limit
}

// TotalPages returns the number of pages for a total item count.
func TotalPages(totalItems int64, limit int) int {
	if totalItems <= 0 || limit <= 0 {
		return 0
	}
	return int(math.Ceil(float64(totalItems) / float64(limit)))
}

// Offset converts page and limit to a SQL offset.
func Offset(page, limit int) int {
	page, limit = NormalizePagination(page, limit)
	return (page - 1) * limit
}
