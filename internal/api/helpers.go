package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// idParam extracts the :id parameter as an int or aborts with BadRequest.
func idParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return 0, false
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
