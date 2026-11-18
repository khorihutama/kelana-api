package query

import (
	"net/url"
	"testing"

	"github.com/khorihutama/hsi-go-api/internal/common/errs"
)

func TestParseFullFilter(t *testing.T) {
	values := url.Values{
		"q":             {"bali"},
		"category":      {"open"},
		"destination":   {"Bali"},
		"departure":     {"Jakarta"},
		"date_from":     {"2026-12-01"},
		"date_to":       {"2027-01-31"},
		"price_min_idr": {"1000000"},
		"price_max_idr": {"5000000"},
		"sort":          {"price_asc"},
		"page":          {"2"},
		"limit":         {"5"},
	}
	f, appErr := Parse(values)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if f.Category != "OPEN" || f.Sort != "price_asc" || f.Page != 2 || f.Limit != 5 {
		t.Fatalf("unexpected filter: %+v", f)
	}
	if f.PriceMinIDR == nil || *f.PriceMinIDR != 1000000 || f.PriceMaxIDR == nil || *f.PriceMaxIDR != 5000000 {
		t.Fatalf("unexpected price filter: %+v", f)
	}
	if f.DateFrom == nil || f.DateTo == nil {
		t.Fatal("expected dates parsed")
	}
	if f.Offset() != 5 {
		t.Fatalf("offset = %d; want 5", f.Offset())
	}
}

func TestParseDefaults(t *testing.T) {
	f, appErr := Parse(url.Values{})
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if f.Page != 1 || f.Limit != 10 || f.Sort != "newest" || f.Category != "" {
		t.Fatalf("unexpected defaults: %+v", f)
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name   string
		values url.Values
		field  string
	}{
		{"category", url.Values{"category": {"VIP"}}, "category"},
		{"date format", url.Values{"date_from": {"01-12-2026"}}, "date_from"},
		{"date order", url.Values{"date_from": {"2026-12-10"}, "date_to": {"2026-12-01"}}, "date_to"},
		{"price negative", url.Values{"price_min_idr": {"-1"}}, "price_min_idr"},
		{"price order", url.Values{"price_min_idr": {"9000000"}, "price_max_idr": {"1000000"}}, "price_max_idr"},
		{"sort", url.Values{"sort": {"cheapest"}}, "sort"},
		{"page", url.Values{"page": {"0"}}, "page"},
		{"limit", url.Values{"limit": {"101"}}, "limit"},
	}
	for _, tc := range cases {
		_, appErr := Parse(tc.values)
		if appErr == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
		if len(appErr.Details) == 0 || appErr.Details[0].Field != tc.field {
			t.Fatalf("%s: expected field %s, got %+v", tc.name, tc.field, appErr.Details)
		}
		if appErr.Code != errs.CodeValidation {
			t.Fatalf("%s: expected %s, got %s", tc.name, errs.CodeValidation, appErr.Code)
		}
	}
}
