package services

import (
	"net/http"
 
 	"testcenter-server/models"
 	"github.com/gin-gonic/gin"
 )

// Gin Handlers for Config

var ConfigServiceInstance *ConfigService

func InitConfigService(projectsFile, devicesFile string) {
	ConfigServiceInstance = NewConfigService(projectsFile, devicesFile)
}

func GetProjectsHandler(c *gin.Context) {
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func SaveProjectHandler(c *gin.Context) {
	var p models.Project
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.ID == "" {
		p.ID = p.ProjectCode
	}
	if err := ConfigServiceInstance.UpdateProject(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func DeleteProjectHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing project id"})
		return
	}
	if err := ConfigServiceInstance.DeleteProject(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetDevicesHandler(c *gin.Context) {
	devices, err := ConfigServiceInstance.GetAllDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func SaveDeviceHandler(c *gin.Context) {
	var d models.Device
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ConfigServiceInstance.UpdateDevice(d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func DeleteDeviceHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing device id"})
		return
	}
	if err := ConfigServiceInstance.DeleteDevice(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
