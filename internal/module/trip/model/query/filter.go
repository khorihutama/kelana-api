package query

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
	"github.com/khorihutama/hsi-go-api/internal/common/util"
)

var allowedSorts = map[string]string{
	"newest":        "newest",
	"price_asc":     "price_asc",
	"price_desc":    "price_desc",
	"rating_desc":   "rating_desc",
	"departure_asc": "departure_asc",
}

type Filter struct {
	Query       string
	Category    string
	Destination string
	Departure   string
	DateFrom    *time.Time
	DateTo      *time.Time
	PriceMinIDR *int64
	PriceMaxIDR *int64
	Sort        string
	Page        int
	Limit       int
}

func Parse(values url.Values) (Filter, *errs.AppError) {
	f := Filter{
		Query:       strings.TrimSpace(values.Get("q")),
		Category:    strings.ToUpper(strings.TrimSpace(values.Get("category"))),
		Destination: strings.TrimSpace(values.Get("destination")),
		Departure:   strings.TrimSpace(values.Get("departure")),
		Sort:        strings.TrimSpace(values.Get("sort")),
		Page:        1,
		Limit:       10,
	}
	if len(f.Query) > 100 {
		return f, invalid("q", "q maksimal 100 karakter")
	}
	if len(f.Destination) > 100 {
		return f, invalid("destination", "destination maksimal 100 karakter")
	}
	if len(f.Departure) > 100 {
		return f, invalid("departure", "departure maksimal 100 karakter")
	}
	if f.Category != "" {
		switch f.Category {
		case "OPEN", "PRIVATE", "EDU":
		default:
			return f, invalid("category", "category harus salah satu dari OPEN, PRIVATE, EDU")
		}
	}

	var appErr *errs.AppError
	if f.DateFrom, appErr = parseDate(values.Get("date_from"), "date_from"); appErr != nil {
		return f, appErr
	}
	if f.DateTo, appErr = parseDate(values.Get("date_to"), "date_to"); appErr != nil {
		return f, appErr
	}
	if f.DateFrom != nil && f.DateTo != nil && f.DateFrom.After(*f.DateTo) {
		return f, invalid("date_to", "date_to harus sama atau setelah date_from")
	}

	if f.PriceMinIDR, appErr = parsePrice(values.Get("price_min_idr"), "price_min_idr"); appErr != nil {
		return f, appErr
	}
	if f.PriceMaxIDR, appErr = parsePrice(values.Get("price_max_idr"), "price_max_idr"); appErr != nil {
		return f, appErr
	}
	if f.PriceMinIDR != nil && f.PriceMaxIDR != nil && *f.PriceMinIDR > *f.PriceMaxIDR {
		return f, invalid("price_max_idr", "price_max_idr harus lebih besar atau sama dengan price_min_idr")
	}

	if f.Sort == "" {
		f.Sort = "newest"
	}
	if _, ok := allowedSorts[f.Sort]; !ok {
		return f, invalid("sort", "sort harus salah satu dari newest, price_asc, price_desc, rating_desc, departure_asc")
	}

	if f.Page, appErr = parsePositive(values.Get("page"), "page", 1); appErr != nil {
		return f, appErr
	}
	if f.Limit, appErr = parsePositive(values.Get("limit"), "limit", 10); appErr != nil {
		return f, appErr
	}
	if f.Limit > 100 {
		return f, invalid("limit", "limit maksimal 100")
	}
	return f, nil
}

func (f Filter) Offset() int {
	return util.Offset(f.Page, f.Limit)
}

func invalid(field, message string) *errs.AppError {
	return errs.Validation([]errs.FieldError{{Field: field, Message: message}})
}

func parseDate(raw, field string) (*time.Time, *errs.AppError) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, invalid(field, field+" harus berformat YYYY-MM-DD")
	}
	return &parsed, nil
}

func parsePrice(raw, field string) (*int64, *errs.AppError) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, invalid(field, field+" harus bilangan bulat >= 0")
	}
	return &value, nil
}

func parsePositive(raw, field string, fallback int) (int, *errs.AppError) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, invalid(field, field+" harus bilangan bulat >= 1")
	}
	return value, nil
}
