package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"era/booru/internal/db"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterSettingsRoutes exposes endpoints for managing global settings.
func RegisterSettingsRoutes(r *gin.Engine, pool *pgxpool.Pool) {
	group := r.Group("/api/settings")
	group.GET("/hidden-tags", listHiddenTagFiltersHandler(pool))
	group.POST("/hidden-tags", createHiddenTagFilterHandler(pool))
	group.POST("/hidden-tags/:id/select", selectHiddenTagFilterHandler(pool))
	group.DELETE("/hidden-tags/:id", deleteHiddenTagFilterHandler(pool))
}

func listHiddenTagFiltersHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		filters, err := db.ListHiddenTagFilters(c.Request.Context(), pool)
		if err != nil {
			log.Printf("list hidden filters: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		active, err := db.ActiveHiddenTagFilter(c.Request.Context(), pool)
		if err != nil {
			log.Printf("active hidden filter: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		activeID := active.ID
		activeValue := active.Value
		outFilters := make([]gin.H, 0, len(filters))
		for _, filter := range filters {
			outFilters = append(outFilters, gin.H{
				"id":         filter.ID,
				"value":      filter.Value,
				"is_default": filter.Value == "",
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"filters": outFilters,
			"active": gin.H{
				"id":    activeID,
				"value": activeValue,
			},
		})
	}
}

func createHiddenTagFilterHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	type request struct {
		Value string `json:"value"`
	}
	return func(c *gin.Context) {
		var body request
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		filter, err := db.CreateHiddenTagFilter(c.Request.Context(), pool, body.Value)
		if err != nil {
			if err.Error() == "hidden tag filter cannot be empty" {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if db.IsConflict(err) {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "hidden tag filter already exists"})
				return
			}
			log.Printf("create hidden filter: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"filter": gin.H{
				"id":         filter.ID,
				"value":      filter.Value,
				"is_default": filter.Value == "",
			},
		})
	}
}

func selectHiddenTagFilterHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := db.SetActiveHiddenTagFilter(c.Request.Context(), pool, int64(id)); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("select hidden filter %d: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		active, err := db.ActiveHiddenTagFilter(c.Request.Context(), pool)
		if err != nil {
			log.Printf("read active hidden filter: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"active": gin.H{
				"id":    active.ID,
				"value": active.Value,
			},
		})
	}
}

func deleteHiddenTagFilterHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := db.DeleteHiddenTagFilter(c.Request.Context(), pool, int64(id)); err != nil {
			if errors.Is(err, db.ErrDefaultFilter) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			log.Printf("delete hidden filter %d: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
