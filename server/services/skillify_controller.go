package services

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ScanRequest struct {
	URL string `json:"url" binding:"required"`
}

type GenerateRequest struct {
	Elements []ElementData `json:"elements" binding:"required"`
}

// SkillifyScanHandler triggers the automated page scanning
func SkillifyScanHandler(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	results, err := SkillifyServiceInstance.ScanPageElements(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// SkillifyGenerateHandler triggers AI Skill generation from elements
func SkillifyGenerateHandler(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Elements are required"})
		return
	}

	skills, err := SkillifyServiceInstance.GenerateSkillsWithAI(req.Elements)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"skills": skills})
}

// SkillifySaveHandler persists generated skills
func SkillifySaveHandler(c *gin.Context) {
	var req struct {
		Skills    []SkillNode `json:"skills" binding:"required"`
		SuiteName string      `json:"suite_name"`
		SourceURL string      `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Skills are required"})
		return
	}

	err := SkillifyServiceInstance.SaveSkillsToLibrary(req.Skills, req.SuiteName, req.SourceURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Skills saved successfully"})
}
