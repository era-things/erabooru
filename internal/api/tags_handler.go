package api

import (
	"log"
	"net/http"
	"strings"

	"era/booru/internal/db"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tagSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func RegisterTagRoutes(r *gin.Engine, pool *pgxpool.Pool) {
	r.GET("/api/tags", listTagsHandler(pool))
	r.GET("/api/tags/suggest", suggestTagsHandler(pool))
}

func listTagsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tags, err := db.ListTags(c.Request.Context(), pool, "", 0)
		if err != nil {
			log.Printf("list tags: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"tags": makeTagSummaries(tags)})
	}
}

func suggestTagsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		prefix := strings.TrimSpace(c.Query("q"))
		if prefix == "" {
			c.JSON(http.StatusOK, gin.H{"tags": []tagSummary{}})
			return
		}

		tags, err := db.ListTags(c.Request.Context(), pool, prefix, 10)
		if err != nil {
			log.Printf("suggest tags %q: %v", prefix, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{"tags": makeTagSummaries(tags)})
	}
}

func makeTagSummaries(tags []db.Tag) []tagSummary {
	summaries := make([]tagSummary, 0, len(tags))
	for _, t := range tags {
		summaries = append(summaries, tagSummary{Name: t.Name, Count: t.Count})
	}
	return summaries
}
