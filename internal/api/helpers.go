package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// idParam extracts the :id parameter and validates it or aborts the request with a 400 Bad Request status.
func idParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if len(id) != 32 {
		c.AbortWithStatus(http.StatusBadRequest)
		return "Filename length should be exactly 32 characters", false
	}
	// Check if it's valid hex
	for _, ch := range id {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			c.AbortWithStatus(http.StatusBadRequest)
			return "Invalid filename: must be a hexadecimal string", false
		}
	}
	return id, true
}

// bindJSONOrAbort binds JSON body into dst or aborts with 400 status.
func bindJSONOrAbort(c *gin.Context, dst any) bool {
	if err := c.BindJSON(dst); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return false
	}
	return true
}

// bindIDAndJSON extracts an ID and binds the JSON body.
func bindIDAndJSON[T any](c *gin.Context, dst *T) (string, bool) {
	id, ok := idParam(c)
	if !ok {
		return "", false
	}
	if !bindJSONOrAbort(c, dst) {
		return "", false
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
