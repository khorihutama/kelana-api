//go:build integration

package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"

	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/database"
	"github.com/khorihutama/hsi-go-api/internal/module/trip/model/query"
)

func newIntegrationRepo(t *testing.T) (*gormRepository, context.Context) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	_ = godotenv.Load(filepath.Join(root, ".env"))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	db, err := database.Connect(cfg.DSN(), false)
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	return &gormRepository{db: db}, context.Background()
}

func TestIntegrationFilterCombination(t *testing.T) {
	repo, ctx := newIntegrationRepo(t)
	maxPrice := int64(3000000)
	trips, total, err := repo.List(ctx, query.Filter{
		Category:    "OPEN",
		PriceMaxIDR: &maxPrice,
		Page:        1,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total == 0 || len(trips) == 0 {
		t.Fatal("expected seeded trips")
	}
	for _, trip := range trips {
		if trip.Category != "OPEN" {
			t.Fatalf("category %s leaked into OPEN filter", trip.Category)
		}
		if trip.BasePriceIDR > maxPrice {
			t.Fatalf("price %d above max %d", trip.BasePriceIDR, maxPrice)
		}
	}
}

func TestIntegrationEmptyFilter(t *testing.T) {
	repo, ctx := newIntegrationRepo(t)
	trips, total, err := repo.List(ctx, query.Filter{
		Query: "tidak-ada-trip-seperti-ini",
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 || len(trips) != 0 {
		t.Fatalf("expected empty result, got total=%d len=%d", total, len(trips))
	}
}

func TestIntegrationPaginationAndPriceSort(t *testing.T) {
	repo, ctx := newIntegrationRepo(t)
	trips, total, err := repo.List(ctx, query.Filter{Sort: "price_asc", Page: 1, Limit: 5})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 24 || len(trips) != 5 {
		t.Fatalf("expected at least 24 trips and page size 5, got total=%d len=%d", total, len(trips))
	}
	for i := 1; i < len(trips); i++ {
		if trips[i].BasePriceIDR < trips[i-1].BasePriceIDR {
			t.Fatalf("price sort broken at index %d", i)
		}
	}
}

func TestIntegrationDetailSchedulesAndStats(t *testing.T) {
	repo, ctx := newIntegrationRepo(t)
	trip, err := repo.GetByIDOrSlug(ctx, "bali-open-trip")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if trip == nil {
		t.Fatal("expected bali-open-trip")
	}
	if len(trip.Itineraries) == 0 || len(trip.Schedules) < 2 || len(trip.HotelOptions) < 2 {
		t.Fatalf("detail relations incomplete: itineraries=%d schedules=%d hotels=%d",
			len(trip.Itineraries), len(trip.Schedules), len(trip.HotelOptions))
	}

	destinations, err := repo.Destinations(ctx)
	if err != nil || len(destinations) < 3 {
		t.Fatalf("destinations: len=%d err=%v", len(destinations), err)
	}
	categories, err := repo.Categories(ctx)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	total := int64(0)
	for _, c := range categories {
		total += c.Count
	}
	if total < 24 {
		t.Fatalf("expected at least 24 trips across categories, got %d", total)
	}
}
