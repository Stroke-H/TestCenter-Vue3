package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// MindNode 定义了思维导图节点结构
type MindNode struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Status   string     `json:"status"`
	IsRoot   bool       `json:"isRoot,omitempty"`
	Children []MindNode `json:"children,omitempty"`
}

// ProcessItem 定义了完整的流程记录结构
type ProcessItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Data      MindNode  `json:"data"`
	UpdatedAt string    `json:"updatedAt"`
}

const dataDir = "./data/processes"

// EnsureDataDir 确保数据目录存在
func EnsureDataDir() {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		_ = os.MkdirAll(dataDir, 0755)
	}
}

// ListProcessesHandler 获取所有流程列表（仅元数据）
func ListProcessesHandler(c *gin.Context) {
	files, err := os.ReadDir(dataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read data directory"})
		return
	}

	var processes []ProcessItem
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(dataDir, file.Name()))
			if err != nil {
				continue
			}
			var p ProcessItem
			if err := json.Unmarshal(data, &p); err == nil {
				// 为了节省带宽，列表请求可以考虑不返回完整的 data 树，但目前数据量小，直接全量返回
				processes = append(processes, p)
			}
		}
	}

	// 如果没有数据，返回空数组
	if processes == nil {
		processes = []ProcessItem{}
	}
	c.JSON(http.StatusOK, processes)
}

// GetProcessHandler 获取单个流程详情
func GetProcessHandler(c *gin.Context) {
	id := c.Param("id")
	filePath := filepath.Join(dataDir, id+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		return
	}

	var p ProcessItem
	if err := json.Unmarshal(data, &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data corruption"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// SaveProcessHandler 保存或更新流程
func SaveProcessHandler(c *gin.Context) {
	var p ProcessItem
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if p.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Process ID is required"})
		return
	}

	// 更新时间戳
	p.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

	filePath := filepath.Join(dataDir, p.ID+".json")
	fileData, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize data"})
		return
	}

	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// DeleteProcessHandler 删除流程
func DeleteProcessHandler(c *gin.Context) {
	id := c.Param("id")
	filePath := filepath.Join(dataDir, id+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Process not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Process %s deleted", id)})
}
