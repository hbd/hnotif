package apimodel

// TopStoriesResponse models a response from the HN top stories endpoint.
type TopStoriesResponse []int

// GetItemRespone models a response from the HN item endpoint.
type GetItemResponse struct {
	By          string `json:"by"`
	Descendants int    `json:"descendants"`
	ID          int    `json:"id"`
	Kids        []int  `json:"kids"`
	Score       int    `json:"score"`
	Time        int64  `json:"time"` // Unix time // epoch.
	Title       string `json:"title"`
	Type        string `json:"type"`
	URL         string `json:"url"`
}
