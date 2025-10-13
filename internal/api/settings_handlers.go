package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"era/booru/ent"
	"era/booru/internal/db"
)

// RegisterSettingsRoutes exposes endpoints for managing global settings.
func RegisterSettingsRoutes(r *gin.Engine, dbClient *ent.Client) {
	group := r.Group("/api/settings")
	group.GET("/hidden-tags", listHiddenTagFiltersHandler(dbClient))
	group.POST("/hidden-tags", createHiddenTagFilterHandler(dbClient))
	group.POST("/hidden-tags/:id/select", selectHiddenTagFilterHandler(dbClient))
	group.DELETE("/hidden-tags/:id", deleteHiddenTagFilterHandler(dbClient))
}

func listHiddenTagFiltersHandler(dbClient *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		filters, err := db.ListHiddenTagFilters(c.Request.Context(), dbClient)
		if err != nil {
			log.Printf("list hidden filters: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		activeValue, err := db.ActiveHiddenTagFilterValue(c.Request.Context(), dbClient)
		if err != nil {
			log.Printf("active hidden filter: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		activeID := 0
		outFilters := make([]gin.H, 0, len(filters))
		for _, filter := range filters {
			outFilters = append(outFilters, gin.H{
				"id":         filter.ID,
				"value":      filter.Value,
				"is_default": filter.Value == "",
			})
			if filter.Value == activeValue {
				activeID = filter.ID
			}
		}

		if activeID == 0 && len(filters) > 0 {
			if active, err := db.ActiveHiddenTagFilter(c.Request.Context(), dbClient); err == nil {
				activeID = active.ID
				activeValue = active.Value
			}
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

func createHiddenTagFilterHandler(dbClient *ent.Client) gin.HandlerFunc {
	type request struct {
		Value string `json:"value"`
	}
	return func(c *gin.Context) {
		var body request
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		filter, err := db.CreateHiddenTagFilter(c.Request.Context(), dbClient, body.Value)
		if err != nil {
			if err.Error() == "hidden tag filter cannot be empty" {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if ent.IsConstraintError(err) {
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

func selectHiddenTagFilterHandler(dbClient *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := db.SetActiveHiddenTagFilter(c.Request.Context(), dbClient, id); err != nil {
			if ent.IsNotFound(err) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("select hidden filter %d: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		active, err := db.ActiveHiddenTagFilter(c.Request.Context(), dbClient)
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

func deleteHiddenTagFilterHandler(dbClient *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := db.DeleteHiddenTagFilter(c.Request.Context(), dbClient, id); err != nil {
			if err.Error() == "cannot delete the default hidden tag filter" {
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
