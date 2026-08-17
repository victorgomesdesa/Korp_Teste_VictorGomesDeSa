package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const healthCheckTimeout = 2 * time.Second

type DatabasePinger interface {
	Ping(context.Context) error
}

type HealthHandler struct {
	database DatabasePinger
}

func NewHealthHandler(database DatabasePinger) *HealthHandler {
	return &HealthHandler{database: database}
}

func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
	defer cancel()

	if err := h.database.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
