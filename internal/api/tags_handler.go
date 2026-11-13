package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/search"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tagSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func RegisterTagRoutes(r *gin.Engine, pool *pgxpool.Pool) {
	r.GET("/api/tags", listTagsHandler(pool))
	r.GET("/api/tags/suggest", suggestTagsHandler(pool))
	r.GET("/api/tags/review", tagReviewHandler(pool, func() string { return embed.ActiveProfile().ID }))
}

func tagReviewHandler(pool *pgxpool.Pool, activeProfile func() string) gin.HandlerFunc {
	cache := newTagReviewCache(128)
	return func(c *gin.Context) {
		tag := strings.TrimSpace(c.Query("tag"))
		if tag == "" || tag == "tagme" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Choose a tag other than tagme."})
			return
		}
		ctx := c.Request.Context()
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			tagLearningError(c, err)
			return
		}
		defer tx.Rollback(ctx)
		key := tagReviewKey{tag: tag, profile: activeProfile(), cursor: c.Query("cursor")}
		// Read the cache revision and active filter in the same snapshot as the
		// ranking queries. A committed edit always has a different revision.
		if err := tx.QueryRow(ctx, `SELECT r.revision,f.value FROM tag_review_state r
 CROSS JOIN app_settings s JOIN hidden_tag_filter f ON f.id=s.active_hidden_filter_id
 WHERE r.singleton AND s.singleton`).Scan(&key.revision, &key.filter); err != nil {
			tagLearningError(c, err)
			return
		}
		if body, ok := cache.get(key); ok {
			c.Data(http.StatusOK, "application/json; charset=utf-8", body)
			return
		}
		profile, filter := key.profile, key.filter
		var examples []db.Media
		// The client keeps representatives while paging through suggestions.
		if c.Query("cursor") == "" {
			examples, _, err = search.TagReviewMedia(ctx, tx, tag, profile, filter, true, 4, "")
			if err != nil {
				tagLearningError(c, err)
				return
			}
		}
		// Return a client error for malformed pagination cursors.
		items, next, err := search.TagReviewMedia(ctx, tx, tag, profile, filter, false, 40, key.cursor)
		if err != nil {
			tagLearningError(c, err)
			return
		}
		body, err := json.Marshal(gin.H{"representatives": mediaResponses(examples, true), "media": mediaResponses(items, true), "next_cursor": next})
		if err != nil {
			tagLearningError(c, err)
			return
		}
		cache.put(key, body)
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	}
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
