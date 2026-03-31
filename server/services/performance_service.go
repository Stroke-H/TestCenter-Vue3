package services

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// ListLighthouseReportsHandler scans the report/ directory and returns existing files
func ListLighthouseReportsHandler(c *gin.Context) {
	// Root of project is .. relative to server/ dir
	reportDir := filepath.Join("..", "report")
	
	files, err := os.ReadDir(reportDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet, return empty list
			c.JSON(http.StatusOK, []string{})
			return
		}
		log.Println("[ERROR] Read report dir failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read report directory"})
		return
	}

	var filenames []string
	for _, file := range files {
		// Only list .html reports for the frontend iframe display
		if !file.IsDir() && filepath.Ext(file.Name()) == ".html" {
			filenames = append(filenames, file.Name())
		}
	}

	c.JSON(http.StatusOK, filenames)
}
