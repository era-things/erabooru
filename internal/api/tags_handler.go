package api

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/tag"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
)

type tagSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func RegisterTagRoutes(r *gin.Engine, db *ent.Client) {
	r.GET("/api/tags", listTagsHandler(db))
	r.GET("/api/tags/suggest", suggestTagsHandler(db))
}

func listTagsHandler(db *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tags, err := db.Tag.Query().
			WithMedia(func(q *ent.MediaQuery) {
				q.Select(media.FieldID)
			}).
			All(ctx)
		if err != nil {
			log.Printf("list tags: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		out := makeTagSummaries(tags)
		sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
		c.JSON(http.StatusOK, gin.H{"tags": out})
	}
}

func suggestTagsHandler(db *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		prefix := strings.TrimSpace(c.Query("q"))
		if prefix == "" {
			c.JSON(http.StatusOK, gin.H{"tags": []tagSummary{}})
			return
		}

		ctx := c.Request.Context()

		tags, err := db.Tag.Query().
			Where(
				tag.NameHasPrefix(prefix),
				tag.HasMedia(),
			).
			WithMedia(func(q *ent.MediaQuery) {
				q.Select(media.FieldID)
			}).
			Order(
				tag.ByMediaCount(sql.OrderDesc()),
				tag.ByName(sql.OrderAsc()),
			).
			Limit(10).
			All(ctx)
		if err != nil {
			log.Printf("suggest tags %q: %v", prefix, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{"tags": makeTagSummaries(tags)})
	}
}

func makeTagSummaries(tags []*ent.Tag) []tagSummary {
	summaries := make([]tagSummary, 0, len(tags))
	for _, t := range tags {
		summaries = append(summaries, tagSummary{Name: t.Name, Count: len(t.Edges.Media)})
	}
	return summaries
}
