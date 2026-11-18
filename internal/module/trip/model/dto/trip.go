package dto

type TripCard struct {
	ID              string  `json:"id"`
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Category        string  `json:"category"`
	DepartureCity   string  `json:"departure_city"`
	DestinationCity string  `json:"destination_city"`
	DurationDays    int     `json:"duration_days"`
	BasePriceIDR    int64   `json:"base_price_idr"`
	Currency        string  `json:"currency"`
	HeroImageURL    *string `json:"hero_image_url,omitempty"`
	RatingAvg       float64 `json:"rating_avg"`
	RatingCount     int     `json:"rating_count"`
	NextDeparture   *string `json:"next_departure,omitempty"`
	SeatsAvailable  *int    `json:"seats_available,omitempty"`
}

type Itinerary struct {
	DayNumber   int    `json:"day_number"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Facility struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type HotelOption struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Address      string   `json:"address"`
	Facilities   []string `json:"facilities"`
	ExtraCostIDR int64    `json:"extra_cost_idr"`
	Currency     string   `json:"currency"`
}

type Schedule struct {
	ID             string `json:"id"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	DeparturePoint string `json:"departure_point"`
	PriceIDR       int64  `json:"price_idr"`
	Currency       string `json:"currency"`
	SeatsAvailable int    `json:"seats_available"`
	Status         string `json:"status"`
}

type TripDetail struct {
	ID                string        `json:"id"`
	Slug              string        `json:"slug"`
	Title             string        `json:"title"`
	Description       string        `json:"description"`
	Category          string        `json:"category"`
	DepartureCity     string        `json:"departure_city"`
	DestinationCity   string        `json:"destination_city"`
	DurationDays      int           `json:"duration_days"`
	BasePriceIDR      int64         `json:"base_price_idr"`
	Currency          string        `json:"currency"`
	HeroImageURL      *string       `json:"hero_image_url,omitempty"`
	RatingAvg         float64       `json:"rating_avg"`
	RatingCount       int           `json:"rating_count"`
	RequiredDocuments []string      `json:"required_documents"`
	Itineraries       []Itinerary   `json:"itineraries"`
	Facilities        []Facility    `json:"facilities"`
	HotelOptions      []HotelOption `json:"hotel_options"`
	Schedules         []Schedule    `json:"schedules"`
}

type Destination struct {
	City  string `json:"city"`
	Count int64  `json:"count"`
}

type Category struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}
