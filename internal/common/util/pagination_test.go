package util

import "testing"

func TestNormalizePagination(t *testing.T) {
	cases := []struct {
		page, limit       int
		wantPage, wantLim int
	}{
		{0, 0, 1, 10},
		{-3, 500, 1, 100},
		{2, 25, 2, 25},
		{1, 100, 1, 100},
	}
	for _, tc := range cases {
		page, limit := NormalizePagination(tc.page, tc.limit)
		if page != tc.wantPage || limit != tc.wantLim {
			t.Fatalf("NormalizePagination(%d,%d) = %d,%d; want %d,%d", tc.page, tc.limit, page, limit, tc.wantPage, tc.wantLim)
		}
	}
}

func TestTotalPages(t *testing.T) {
	if got := TotalPages(0, 10); got != 0 {
		t.Fatalf("TotalPages(0,10) = %d; want 0", got)
	}
	if got := TotalPages(21, 10); got != 3 {
		t.Fatalf("TotalPages(21,10) = %d; want 3", got)
	}
	if got := TotalPages(20, 10); got != 2 {
		t.Fatalf("TotalPages(20,10) = %d; want 2", got)
	}
}

func TestOffset(t *testing.T) {
	if got := Offset(1, 10); got != 0 {
		t.Fatalf("Offset(1,10) = %d; want 0", got)
	}
	if got := Offset(3, 25); got != 50 {
		t.Fatalf("Offset(3,25) = %d; want 50", got)
	}
	if got := Offset(0, 0); got != 0 {
		t.Fatalf("Offset(0,0) = %d; want 0", got)
	}
}
