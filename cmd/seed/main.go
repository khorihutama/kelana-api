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

	trips := tripSeeds()
	for i, t := range trips {
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

		start := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i*2)
		for s := 0; s < 2; s++ {
			scheduleStart := start.AddDate(0, 0, s*45)
			err = db.Exec(`
				INSERT INTO trip_schedules (id, trip_id, start_date, end_date, departure_point, quota_total,
					seats_sold, price_idr, currency, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'IDR', 'OPEN')
				ON CONFLICT (trip_id, start_date) DO NOTHING
			`, uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("schedule:%s:%s", t.Slug, scheduleStart.Format("2006-01-02")))),
				tripID, scheduleStart, scheduleStart.AddDate(0, 0, t.DurationDays-1), t.DepartureCity+" Pusat",
				12+i%12, (i*3+s)%(12+i%12), t.BasePriceIDR).Error
			if err != nil {
				log.Fatalf("seed schedule %s: %v", t.Slug, err)
			}
		}

		days := t.DurationDays
		if days > 3 {
			days = 3
		}
		for day := 1; day <= days; day++ {
			err = db.Exec(`
				INSERT INTO trip_itineraries (id, trip_id, day_number, title, description, sort_order)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (trip_id, day_number) DO NOTHING
			`, uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("itinerary:%s:%d", t.Slug, day))),
				tripID, day, fmt.Sprintf("Hari %d", day),
				"Kegiatan perjalanan, akomodasi, dan waktu bebas sesuai itinerary.", day).Error
			if err != nil {
				log.Fatalf("seed itinerary %s day %d: %v", t.Slug, day, err)
			}
		}

		facilities := []struct {
			Name string
			Type string
		}{
			{"Transportasi", "INCLUDED"},
			{"Makan 3x", "INCLUDED"},
			{"Tiket masuk", "INCLUDED"},
			{"Pengeluaran pribadi", "EXCLUDED"},
		}
		for idx, f := range facilities {
			err = db.Exec(`
				INSERT INTO trip_facilities (id, trip_id, name, type, sort_order)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (trip_id, name) DO NOTHING
			`, uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("facility:%s:%s", t.Slug, f.Name))),
				tripID, f.Name, f.Type, idx).Error
			if err != nil {
				log.Fatalf("seed facility %s: %v", t.Slug, err)
			}
		}
	}

	hotels := []struct {
		Key        string
		Name       string
		Address    string
		Facilities []string
	}{
		{"hotel:a", "Hotel A", "Jl. Pantai Kuta No. 1, Badung", []string{"TV & AC", "WiFi 24 Jam", "Restoran", "Kolam Renang"}},
		{"hotel:b", "Hotel B", "Jl. Raya Ubud No. 8, Gianyar", []string{"TV & AC", "WiFi 24 Jam", "Restoran"}},
		{"hotel:c", "Hotel C", "Jl. Malioboro No. 21, Yogyakarta", []string{"TV & AC", "WiFi 24 Jam"}},
	}
	hotelIDs := make([]uuid.UUID, len(hotels))
	for i, h := range hotels {
		hotelIDs[i] = uuid.NewSHA1(uuid.NameSpaceOID, []byte(h.Key))
		err := db.Exec(`
			INSERT INTO hotels (id, name, address, facilities) VALUES (?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING
		`, hotelIDs[i], h.Name, h.Address, pq.Array(h.Facilities)).Error
		if err != nil {
			log.Fatalf("seed hotel %s: %v", h.Name, err)
		}
	}

	tripIDs := make([]uuid.UUID, len(trips))
	for i, t := range trips {
		tripIDs[i] = uuid.NewSHA1(uuid.NameSpaceOID, []byte("trip:"+t.Slug))
	}
	for i, t := range trips {
		if t.Category == "EDU" {
			continue
		}
		for hIdx, hotelID := range hotelIDs {
			if hIdx == 2 && t.Category != "PRIVATE" {
				continue
			}
			err := db.Exec(`
				INSERT INTO trip_hotels (id, trip_id, hotel_id, extra_cost_idr, sort_order)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (trip_id, hotel_id) DO NOTHING
			`, uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("trip-hotel:%s:%d", t.Slug, hIdx))),
				tripIDs[i], hotelID, int64(hIdx)*150000, hIdx).Error
			if err != nil {
				log.Fatalf("seed trip hotel %s: %v", t.Slug, err)
			}
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

	log.Printf("seed completed: %d trips, 2 schedules each, itineraries, facilities, hotels, promo HSIGO2025", len(trips))
}

func tripSeeds() []tripSeed {
	return []tripSeed{
		{"bali-open-trip", "Bali Open Trip 4D3N", "Open trip Bali untuk solo traveler maupun grup kecil: Ubud, Tanah Lot, dan Nusa Penida.", "OPEN", "Jakarta", "Bali", 4, 2500000, 4.8, 120},
		{"bromo-sunrise", "Bromo Sunrise 3D2N", "Matahari terbit Bromo, savana Teletubbies, dan kawah Ijen dengan transportasi darat.", "OPEN", "Jakarta", "Malang", 3, 1750000, 4.7, 86},
		{"lombok-open-trip", "Lombok Open Trip 4D3N", "Gili Trawangan, Mandalika, dan air terjun Sendang Gile dalam satu paket open trip.", "OPEN", "Jakarta", "Lombok", 4, 2350000, 4.7, 64},
		{"belitung-island-hop", "Belitung Island Hop 3D2N", "Pulau Pasir, Tanjung Tinggi, dan Laskar Pelangi dalam itinerary santai.", "OPEN", "Jakarta", "Belitung", 3, 1950000, 4.6, 51},
		{"labuan-bajo-sailing", "Labuan Bajo Sailing 4D3N", "Sailing Komodo, Padar, dan Pink Beach dengan kapal sharing.", "OPEN", "Jakarta", "Labuan Bajo", 4, 3200000, 4.8, 73},
		{"bandung-weekend", "Bandung Weekend 2D1N", "Weekend singkat ke Lembang, Tangkuban Perahu, dan factory outlet.", "OPEN", "Jakarta", "Bandung", 2, 850000, 4.5, 92},
		{"yogya-heritage-open", "Yogyakarta Heritage 3D2N", "Borobudur, Prambanan, Malioboro, dan kuliner gudeg dalam open trip.", "OPEN", "Jakarta", "Yogyakarta", 3, 1250000, 4.7, 78},
		{"raja-ampat-open", "Raja Ampat Open Trip 5D4N", "Piaynemo, Wayag, dan snorkeling di Raja Ampat dengan homestay.", "OPEN", "Sorong", "Raja Ampat", 5, 6500000, 4.9, 41},
		{"bromo-ijen", "Bromo Ijen 4D3N", "Dua gunung ikonik Jawa Timur: Bromo dan Ijen dengan jeep serta guide lokal.", "OPEN", "Surabaya", "Malang", 4, 2100000, 4.6, 58},
		{"bali-ubud-retreat", "Bali Ubud Retreat 4D3N", "Retreat yoga dan alam di Ubud dengan penginapan boutique.", "OPEN", "Jakarta", "Bali", 4, 3800000, 4.8, 37},
		{"raja-ampat-private", "Raja Ampat Private Charter", "Charter privat 5 hari untuk keluarga: menyelam, island hopping, dan penginapan pilihan.", "PRIVATE", "Sorong", "Raja Ampat", 5, 12500000, 4.9, 24},
		{"bali-private-villa", "Bali Private Villa 5D4N", "Villa privat dengan kolam renang, driver, dan itinerary fleksibel.", "PRIVATE", "Jakarta", "Bali", 5, 9800000, 4.9, 19},
		{"lombok-private-sailing", "Lombok Private Sailing 4D3N", "Kapal privat untuk keluarga kecil, termasuk kapten dan koki.", "PRIVATE", "Jakarta", "Lombok", 4, 14500000, 4.8, 12},
		{"belitung-private", "Belitung Private Trip 3D2N", "Mobil dan guide privat untuk rombongan keluarga.", "PRIVATE", "Jakarta", "Belitung", 3, 5400000, 4.7, 16},
		{"labuan-bajo-private", "Labuan Bajo Private Phinisi", "Phinisi privat 3 hari dengan rute Komodo dan Padar.", "PRIVATE", "Jakarta", "Labuan Bajo", 3, 15500000, 4.9, 9},
		{"bandung-family-private", "Bandung Family Private 3D2N", "Itinerary keluarga dengan kunjungan kebun teh dan taman hiburan.", "PRIVATE", "Jakarta", "Bandung", 3, 4200000, 4.6, 21},
		{"bromo-private-jeep", "Bromo Private Jeep 2D1N", "Jeep privat, hotel, dan guide untuk sunrise Bromo.", "PRIVATE", "Surabaya", "Malang", 2, 3600000, 4.7, 18},
		{"yogya-private-culture", "Yogyakarta Private Culture 4D3N", "Tur budaya privat: keraton, candi, dan desa wisata.", "PRIVATE", "Jakarta", "Yogyakarta", 4, 5100000, 4.8, 14},
		{"yogya-edu-trip", "Yogyakarta Edu Trip 2D1N", "Study tour edukatif ke Candi Borobudur, Prambanan, dan museum untuk rombongan sekolah.", "EDU", "Jakarta", "Yogyakarta", 2, 950000, 4.6, 54},
		{"bandung-edu-science", "Bandung Edu Science 2D1N", "Kunjungan observatorium, museum sains, dan workshop untuk pelajar.", "EDU", "Jakarta", "Bandung", 2, 890000, 4.5, 61},
		{"bali-edu-marine", "Bali Edu Marine 3D2N", "Edukasi ekosistem laut, mangrove, dan konservasi penyu.", "EDU", "Jakarta", "Bali", 3, 1850000, 4.7, 33},
		{"bromo-edu-geology", "Bromo Edu Geology 3D2N", "Belajar geologi vulkanik di Bromo dan Ijen bersama pemandu.", "EDU", "Surabaya", "Malang", 3, 1650000, 4.6, 28},
		{"lombok-edu-eco", "Lombok Edu Eco 3D2N", "Ekowisata dan konservasi di Lombok untuk rombongan sekolah.", "EDU", "Jakarta", "Lombok", 3, 1750000, 4.5, 22},
		{"labuan-bajo-edu-marine", "Labuan Bajo Edu Marine 4D3N", "Edukasi marine biology di Komodo dan Labuan Bajo.", "EDU", "Jakarta", "Labuan Bajo", 4, 2950000, 4.8, 17},
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
