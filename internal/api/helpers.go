package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// idParam extracts the :id parameter as an int or aborts with BadRequest.
func idParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if len(id) != 32 {
		c.AbortWithStatus(http.StatusBadRequest)
		return "", false
	}
	// Optionally, check if it's valid hex
	for _, ch := range id {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			c.AbortWithStatus(http.StatusBadRequest)
			return "", false
		}
	}
	return id, true
}

// normalizeTags trims, deduplicates and returns clean tag values.
func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			clean = append(clean, t)
		}
	}
	return clean
}
