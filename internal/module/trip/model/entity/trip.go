package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Trip struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Slug              string    `gorm:"not null;uniqueIndex"`
	Title             string    `gorm:"not null"`
	Description       string    `gorm:"not null"`
	Category          string    `gorm:"not null"`
	DepartureCity     string    `gorm:"not null"`
	DestinationCity   string    `gorm:"not null"`
	DurationDays      int       `gorm:"not null"`
	BasePriceIDR      int64     `gorm:"column:base_price_idr;not null"`
	Currency          string    `gorm:"type:char(3);not null;default:IDR"`
	Status            string    `gorm:"not null;default:DRAFT"`
	HeroImageURL      *string
	WaGroupURL        *string
	RequiredDocuments pq.StringArray `gorm:"type:text[];not null"`
	RatingAvg         float64        `gorm:"type:numeric(3,2);not null;default:0"`
	RatingCount       int            `gorm:"not null;default:0"`
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Itineraries  []TripItinerary `gorm:"foreignKey:TripID"`
	Facilities   []TripFacility  `gorm:"foreignKey:TripID"`
	Schedules    []TripSchedule  `gorm:"foreignKey:TripID"`
	HotelOptions []TripHotel     `gorm:"foreignKey:TripID"`
}

func (Trip) TableName() string { return "trips" }

type TripSchedule struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TripID         uuid.UUID `gorm:"type:uuid;not null"`
	StartDate      time.Time `gorm:"type:date;not null"`
	EndDate        time.Time `gorm:"type:date;not null"`
	DeparturePoint string    `gorm:"not null"`
	QuotaTotal     int       `gorm:"not null"`
	SeatsSold      int       `gorm:"not null;default:0"`
	PriceIDR       int64     `gorm:"column:price_idr;not null"`
	Currency       string    `gorm:"type:char(3);not null;default:IDR"`
	Status         string    `gorm:"not null;default:OPEN"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (TripSchedule) TableName() string { return "trip_schedules" }

type TripItinerary struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	TripID      uuid.UUID `gorm:"type:uuid;not null"`
	DayNumber   int       `gorm:"not null"`
	Title       string    `gorm:"not null"`
	Description string    `gorm:"not null"`
	SortOrder   int       `gorm:"not null;default:0"`
}

func (TripItinerary) TableName() string { return "trip_itineraries" }

type TripFacility struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	TripID    uuid.UUID `gorm:"type:uuid;not null"`
	Name      string    `gorm:"not null"`
	Type      string    `gorm:"not null"`
	SortOrder int       `gorm:"not null;default:0"`
}

func (TripFacility) TableName() string { return "trip_facilities" }

type Hotel struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name       string         `gorm:"not null"`
	Address    string         `gorm:"not null"`
	Facilities pq.StringArray `gorm:"type:text[];not null"`
}

func (Hotel) TableName() string { return "hotels" }

type TripHotel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	TripID       uuid.UUID `gorm:"type:uuid;not null"`
	HotelID      uuid.UUID `gorm:"type:uuid;not null"`
	ExtraCostIDR int64     `gorm:"column:extra_cost_idr;not null"`
	SortOrder    int       `gorm:"not null;default:0"`
	Hotel        Hotel     `gorm:"foreignKey:HotelID"`
}

func (TripHotel) TableName() string { return "trip_hotels" }
