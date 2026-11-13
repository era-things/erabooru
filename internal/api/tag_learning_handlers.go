package api

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/queue"
	"era/booru/internal/search"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

func RegisterTagLearningRoutes(r *gin.Engine, pool *pgxpool.Pool, client *river.Client[pgx.Tx]) {
	r.GET("/api/media/:id/tag-recommendations", tagRecommendationsHandler(pool))
	r.POST("/api/media/:id/tag-feedback", tagFeedbackHandler(pool))
	r.GET("/api/settings/tag-statistics", tagStatisticsStatusHandler(pool))
	r.POST("/api/settings/tag-statistics/rebuild", func(c *gin.Context) {
		profile := c.Query("legacy_profile")
		if profile != "" && profile != embed.ActiveProfile().ID {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "Embedding profile changed. Refresh settings."})
			return
		}
		if err := queue.Enqueue(c.Request.Context(), client, queue.RebuildTagStatisticsArgs{LegacyProfile: profile}); err != nil {
			log.Printf("enqueue tag statistics rebuild: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusAccepted)
	})
}

func tagRecommendationsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			return
		}
		result, err := db.RecommendTags(c.Request.Context(), pool, id)
		if err != nil {
			tagLearningError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func tagFeedbackHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Tag    string `json:"tag"`
			Action string `json:"action"`
		}
		id, ok := bindIDAndJSON(c, &body)
		if !ok {
			return
		}
		if err := db.SetTagFeedback(c.Request.Context(), pool, id, strings.TrimSpace(body.Tag), body.Action); err != nil {
			tagLearningError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func tagLearningError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, search.ErrInvalidCursor):
		c.AbortWithStatus(http.StatusBadRequest)
	case errors.Is(err, db.ErrInvalidTagFeedback):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, pgx.ErrNoRows):
		c.AbortWithStatus(http.StatusNotFound)
	default:
		log.Printf("tag learning: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func tagStatisticsStatusHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		state := "idle"
		err := pool.QueryRow(ctx, `SELECT state::text FROM river_job WHERE kind=$1 ORDER BY id DESC LIMIT 1`, (queue.RebuildTagStatisticsArgs{}).Kind()).Scan(&state)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			tagLearningError(c, err)
			return
		}
		var rebuiltAt *time.Time
		var tags, legacy int64
		if err := pool.QueryRow(ctx, `SELECT classifier_rebuilt_at,(SELECT count(DISTINCT tag_id) FROM tag_statistics WHERE positive_count>0),(SELECT count(*) FROM media_embedding WHERE space_id='vision' AND profile='' AND status='ready') FROM tag_statistics_state`).Scan(&rebuiltAt, &tags, &legacy); err != nil {
			tagLearningError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"state": state, "rebuilt_at": rebuiltAt, "tags": tags, "legacy_embeddings": legacy, "active_profile": embed.ActiveProfile().ID})
	}
}
