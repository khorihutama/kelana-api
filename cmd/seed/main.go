package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/database"
)

type tripSeed struct {
	Slug            string
	Title           string
	Description     string
	Category        string
	DepartureCity   string
	DestinationCity string
	DurationDays    int
	BasePriceIDR    int64
	RatingAvg       float64
	RatingCount     int
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.SeedEnabled {
		log.Println("seed skipped: SEED_ENABLED is not true")
		return
	}

	db, err := database.Connect(cfg.DSN(), false)
	if err != nil {
		log.Fatal(err)
	}

	trips := []tripSeed{
		{"bali-open-trip", "Bali Open Trip 4D3N", "Open trip Bali untuk solo traveler maupun grup kecil: Ubud, Tanah Lot, dan Nusa Penida.", "OPEN", "Jakarta", "Bali", 4, 2500000, 4.8, 120},
		{"bromo-sunrise", "Bromo Sunrise 3D2N", "Menikmati matahari terbit Bromo dan savana Teletubbies dengan transportasi darat dari Jakarta.", "OPEN", "Jakarta", "Malang", 3, 1750000, 4.7, 86},
		{"raja-ampat-private", "Raja Ampat Private Charter", "Charter privat 5 hari untuk keluarga: menyelam, island hopping, dan penginapan pilihan.", "PRIVATE", "Sorong", "Raja Ampat", 5, 12500000, 4.9, 24},
		{"yogya-edu-trip", "Yogyakarta Edu Trip 2D1N", "Study tour edukatif ke Candi Borobudur, Prambanan, dan museum untuk rombongan sekolah.", "EDU", "Jakarta", "Yogyakarta", 2, 950000, 4.6, 54},
	}

	for _, t := range trips {
		tripID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("trip:"+t.Slug))
		err := db.Exec(`
			INSERT INTO trips (id, slug, title, description, category, departure_city, destination_city,
				duration_days, base_price_idr, currency, status, required_documents, rating_avg, rating_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'IDR', 'PUBLISHED', '{KTP}', ?, ?)
			ON CONFLICT (slug) DO NOTHING
		`, tripID, t.Slug, t.Title, t.Description, t.Category, t.DepartureCity, t.DestinationCity,
			t.DurationDays, t.BasePriceIDR, t.RatingAvg, t.RatingCount).Error
		if err != nil {
			log.Fatalf("seed trip %s: %v", t.Slug, err)
		}
	}

	schedules := []struct {
		Slug  string
		Start time.Time
		End   time.Time
		Quota int
		Sold  int
	}{
		{"bali-open-trip", date(2026, 12, 10), date(2026, 12, 13), 20, 4},
		{"bali-open-trip", date(2027, 1, 14), date(2027, 1, 17), 20, 0},
		{"bromo-sunrise", date(2026, 12, 18), date(2026, 12, 20), 30, 12},
		{"raja-ampat-private", date(2027, 2, 5), date(2027, 2, 9), 8, 2},
		{"yogya-edu-trip", date(2026, 12, 7), date(2026, 12, 8), 40, 0},
	}

	for _, s := range schedules {
		var tripIDStr string
		if err := db.Raw("SELECT id FROM trips WHERE slug = ?", s.Slug).Scan(&tripIDStr).Error; err != nil {
			log.Fatalf("lookup trip %s: %v", s.Slug, err)
		}
		tripID, err := uuid.Parse(tripIDStr)
		if err != nil {
			log.Fatalf("parse trip id %s: %v", s.Slug, err)
		}
		scheduleID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("schedule:%s:%s", s.Slug, s.Start.Format("2006-01-02"))))
		price := int64(0)
		if err := db.Raw("SELECT base_price_idr FROM trips WHERE id = ?", tripID).Scan(&price).Error; err != nil {
			log.Fatalf("price %s: %v", s.Slug, err)
		}
		err = db.Exec(`
			INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total,
				seats_sold, price_idr, currency, status)
			VALUES (?, ?, ?, ?, 'Jakarta Pusat', ?, ?, ?, 'IDR', 'OPEN')
			ON CONFLICT (trip_id, start_date) DO NOTHING
		`, scheduleID, tripID, s.Start, s.End, s.Quota, s.Sold, price).Error
		if err != nil {
			log.Fatalf("seed schedule %s: %v", s.Slug, err)
		}
	}

	hotels := []struct {
		ID         uuid.UUID
		Name       string
		Address    string
		Facilities []string
	}{
		{uuid.NewSHA1(uuid.NameSpaceOID, []byte("hotel:a")), "Hotel A", "Jl. Pantai Kuta No. 1, Badung", []string{"TV & AC", "WiFi 24 Jam", "Restoran", "Kolam Renang"}},
		{uuid.NewSHA1(uuid.NameSpaceOID, []byte("hotel:b")), "Hotel B", "Jl. Raya Ubud No. 8, Gianyar", []string{"TV & AC", "WiFi 24 Jam", "Restoran"}},
	}
	for _, h := range hotels {
		err := db.Exec(`
			INSERT INTO hotels (id, name, address, facilities) VALUES (?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING
		`, h.ID, h.Name, h.Address, pq.Array(h.Facilities)).Error
		if err != nil {
			log.Fatalf("seed hotel %s: %v", h.Name, err)
		}
	}

	var baliIDStr string
	if err := db.Raw("SELECT id FROM trips WHERE slug = 'bali-open-trip'").Scan(&baliIDStr).Error; err != nil {
		log.Fatal(err)
	}
	baliID, err := uuid.Parse(baliIDStr)
	if err != nil {
		log.Fatal(err)
	}
	for i, h := range hotels {
		tripHotelID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("trip-hotel:%s:%d", baliID, i)))
		err := db.Exec(`
			INSERT INTO trip_hotels (id, trip_id, hotel_id, extra_cost_idr, sort_order)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (trip_id, hotel_id) DO NOTHING
		`, tripHotelID, baliID, h.ID, int64(i)*150000, i).Error
		if err != nil {
			log.Fatalf("seed trip hotel: %v", err)
		}
	}

	promoID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("promo:HSIGO2025"))
	err = db.Exec(`
		INSERT INTO promo_codes (id, code, discount_type, discount_value, min_transaction_idr,
			quota, used_count, valid_from, valid_to, active)
		VALUES (?, 'HSIGO2025', 'PERCENT', 1000, 1000000, 100, 0, ?, ?, true)
		ON CONFLICT (code) DO NOTHING
	`, promoID, date(2026, 1, 1), date(2027, 12, 31)).Error
	if err != nil {
		log.Fatalf("seed promo: %v", err)
	}

	log.Printf("seed completed: %d trips, %d schedules, promo HSIGO2025", len(trips), len(schedules))
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
