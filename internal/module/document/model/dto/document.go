package dto

type UploadResponse struct {
	ID            string `json:"id"`
	ReservationID string `json:"reservation_id"`
	ParticipantID string `json:"participant_id"`
	Type          string `json:"type"`
	SizeBytes     int64  `json:"size_bytes"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type URLResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
	ExpiresAt   string `json:"expires_at"`
}

type MissingDocument struct {
	ParticipantID string `json:"participant_id"`
	FullName      string `json:"full_name"`
	Type          string `json:"type"`
}

type ParticipantCompleteness struct {
	ParticipantID string   `json:"participant_id"`
	FullName      string   `json:"full_name"`
	Uploaded      []string `json:"uploaded"`
	Missing       []string `json:"missing"`
}

type CompletenessResponse struct {
	ReservationID     string                    `json:"reservation_id"`
	RequiredDocuments []string                  `json:"required_documents"`
	Complete          bool                      `json:"complete"`
	Participants      []ParticipantCompleteness `json:"participants"`
	Missing           []MissingDocument         `json:"missing"`
}
