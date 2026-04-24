package services

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"testcenter-server/models"
)

// Gin Handlers for Config

var ConfigServiceInstance *ConfigService

func InitConfigService() {
	ConfigServiceInstance = NewConfigService()
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

func GetAccountsHandler(c *gin.Context) {
	accounts, err := ListAccountProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func SaveAccountHandler(c *gin.Context) {
	var req AccountUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := UpdateAccountProfile(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, account)
}

func GetSandboxAccountsHandler(c *gin.Context) {
	accounts, err := ListSandboxAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func SaveSandboxAccountHandler(c *gin.Context) {
	var account models.SandboxAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if account.Account == "" || account.Password == "" || account.ProjectCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account, password and project_code are required"})
		return
	}
	if account.AccountType == "" {
		account.AccountType = "sandbox"
	}

	saved, err := CreateSandboxAccount(account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, saved)
}

func DeleteSandboxAccountHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing sandbox account id"})
		return
	}
	if err := DeleteSandboxAccount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func UpdateSandboxAccountHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing sandbox account id"})
		return
	}

	var account models.SandboxAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prefer route id as the source of truth.
	account.ID = id

	if account.Account == "" || account.Password == "" || account.ProjectCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account, password and project_code are required"})
		return
	}

	if account.AccountType == "" {
		account.AccountType = "sandbox"
	}

	updated, err := UpdateSandboxAccount(account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}
