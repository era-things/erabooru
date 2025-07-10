package api

import (
	"log"
	"net/http"
	"sort"

	"era/booru/ent"
	"era/booru/ent/tag"

	"github.com/gin-gonic/gin"
)

func RegisterTagRoutes(r *gin.Engine, db *ent.Client) {
	r.GET("/api/tags", listTagsHandler(db))
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
