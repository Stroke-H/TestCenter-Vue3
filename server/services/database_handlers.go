package services

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"testcenter-server/database"
)

var DatabaseManager = database.NewManager()

func GetDatabaseConfigHandler(c *gin.Context) {
	config := database.LoadConfigFromEnv()
	c.JSON(http.StatusOK, config.Safe())
}

func GetDatabaseHealthHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()

	status := DatabaseManager.Health(ctx)
	httpStatus := http.StatusOK
	if !status.Configured {
		httpStatus = http.StatusPreconditionFailed
	} else if !status.Connected {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}
