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
