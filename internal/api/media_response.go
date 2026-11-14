package api

import "era/booru/internal/db"

type mediaResponse struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Format       string `json:"format"`
}

// Search, similarity, and tag review expose the same media fields.
func mediaResponses(items []db.Media, previews bool) []mediaResponse {
	out := make([]mediaResponse, len(items))
	for i, m := range items {
		out[i] = mediaResponse{
			ID: m.ID, OriginalName: m.OriginalName, URL: mediaURL(m.ID, previews),
			Width: m.Width, Height: m.Height, Format: m.Format,
		}
	}
	return out
}
