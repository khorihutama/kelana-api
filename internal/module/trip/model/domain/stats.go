package domain

type DestinationCount struct {
	City  string `json:"city"`
	Count int64  `json:"count"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}
