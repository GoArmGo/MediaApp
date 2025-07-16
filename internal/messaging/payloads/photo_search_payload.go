package payloads

// PhotoSearchPayload представляет данные, необходимые для поиска и сохранения фотографий
// через RabbitMQ.
type PhotoSearchPayload struct {
	Query   string `json:"query"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}
