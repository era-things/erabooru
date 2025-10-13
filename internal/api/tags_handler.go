package api

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"era/booru/ent"
	"era/booru/ent/tag"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
)

func RegisterTagRoutes(r *gin.Engine, db *ent.Client) {
	r.GET("/api/tags", listTagsHandler(db))
	r.GET("/api/tags/suggest", suggestTagsHandler(db))
}

func listTagsHandler(db *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tags, err := db.Tag.Query().All(ctx)
		if err != nil {
			log.Printf("list tags: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		out := make([]struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}, len(tags))
		for i, t := range tags {
			count, err := db.Tag.Query().Where(tag.IDEQ(t.ID)).QueryMedia().Count(ctx)
			if err != nil {
				log.Printf("count tag %s: %v", t.Name, err)
				count = 0
			}
			out[i].Name = t.Name
			out[i].Count = count
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
		c.JSON(http.StatusOK, gin.H{"tags": out})
	}
}

func suggestTagsHandler(db *ent.Client) gin.HandlerFunc {
	type tagSuggestion struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	return func(c *gin.Context) {
		prefix := strings.TrimSpace(c.Query("q"))
		if prefix == "" {
			c.JSON(http.StatusOK, gin.H{"tags": []tagSuggestion{}})
			return
		}

		ctx := c.Request.Context()

		tags, err := db.Tag.Query().
			Where(
				tag.NameHasPrefix(prefix),
				tag.HasMedia(),
			).
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

		suggestions := make([]tagSuggestion, 0, len(tags))
		for _, t := range tags {
			count, err := db.Tag.Query().Where(tag.IDEQ(t.ID)).QueryMedia().Count(ctx)
			if err != nil {
				log.Printf("count tag %s: %v", t.Name, err)
				continue
			}
			suggestions = append(suggestions, tagSuggestion{Name: t.Name, Count: count})
		}

		c.JSON(http.StatusOK, gin.H{"tags": suggestions})
	}
}
