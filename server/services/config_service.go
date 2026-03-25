package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"testcenter-server/models"
)

type ConfigService struct {
	projectsFile string
	devicesFile  string
	mu           sync.RWMutex
}

func NewConfigService(projectsFile, devicesFile string) *ConfigService {
	return &ConfigService{
		projectsFile: projectsFile,
		devicesFile:  devicesFile,
	}
}

// Project Methods

func (s *ConfigService) GetAllProjects() ([]models.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Open(s.projectsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Project{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var projects []models.Project
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var p models.Project
		if err := json.Unmarshal([]byte(line), &p); err == nil {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

func (s *ConfigService) SaveProjects(projects []models.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Create(s.projectsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, p := range projects {
		line, err := json.Marshal(p)
		if err != nil {
			continue
		}
		writer.WriteString(string(line) + "\n")
	}
	return writer.Flush()
}

// Device Methods

func (s *ConfigService) GetAllDevices() ([]models.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Open(s.devicesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Device{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var devices []models.Device
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var d models.Device
		if err := json.Unmarshal([]byte(line), &d); err == nil {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (s *ConfigService) SaveDevices(devices []models.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Create(s.devicesFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, d := range devices {
		line, err := json.Marshal(d)
		if err != nil {
			continue
		}
		writer.WriteString(string(line) + "\n")
	}
	return writer.Flush()
}

func (s *ConfigService) UpdateProject(updated models.Project) error {
	projects, err := s.GetAllProjects()
	if err != nil {
		return err
	}

	found := false
	for i, p := range projects {
		if p.ID == updated.ID {
			projects[i] = updated
			found = true
			break
		}
	}

	if !found {
		updated.CreatedAt = time.Now().Format("2006-01-02T15:04:05")
		projects = append(projects, updated)
	}

	return s.SaveProjects(projects)
}

func (s *ConfigService) DeleteProject(id string) error {
	projects, err := s.GetAllProjects()
	if err != nil {
		return err
	}

	var newList []models.Project
	for _, p := range projects {
		if p.ID != id {
			newList = append(newList, p)
		}
	}

	return s.SaveProjects(newList)
}

func (s *ConfigService) UpdateDevice(updated models.Device) error {
	devices, err := s.GetAllDevices()
	if err != nil {
		return err
	}

	found := false
	for i, d := range devices {
		if d.ID == updated.ID {
			devices[i] = updated
			found = true
			break
		}
	}

	if !found {
		if updated.ID == "" {
			updated.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		updated.CreatedAt = time.Now().Format("2006-01-02T15:04:05")
		devices = append(devices, updated)
	}

	return s.SaveDevices(devices)
}

func (s *ConfigService) DeleteDevice(id string) error {
	devices, err := s.GetAllDevices()
	if err != nil {
		return err
	}

	var newList []models.Device
	for _, d := range devices {
		if d.ID != id {
			newList = append(newList, d)
		}
	}

	return s.SaveDevices(newList)
}

func (s *ConfigService) GetProjectBySubCode(subCode string) (*models.Project, error) {
	projects, err := s.GetAllProjects()
	if err != nil {
		return nil, err
	}

	search := strings.ToLower(strings.TrimSpace(subCode))
	if search == "" {
		return nil, fmt.Errorf("empty project code")
	}

	// 1. Direct Match: ID, ProjectCode, or ShortCode
	for _, p := range projects {
		if strings.EqualFold(p.ID, search) ||
			strings.EqualFold(p.ProjectCode, search) ||
			(p.ShortCode != "" && strings.EqualFold(p.ShortCode, search)) {
			return &p, nil
		}
	}

	// 2. Pure Numeric ID Support (e.g., "1100" -> "A1100")
	// If search is numeric, check if A+search exists
	if _, err := fmt.Sscanf(search, "%d"); err == nil {
		targetID := "A" + search
		for _, p := range projects {
			if strings.EqualFold(p.ID, targetID) {
				return &p, nil
			}
		}
	}

	// 3. Name Match (Allow whole name or part)
	for _, p := range projects {
		if strings.EqualFold(p.ProjectName, subCode) ||
			strings.Contains(strings.ToLower(p.ProjectName), search) {
			return &p, nil
		}
	}

	// 4. Hardcoded Fallbacks (Legacy support if data not updated yet)
	mapping := map[string]string{
		"swa": "A1100",
		"swi": "A1106",
		"msa": "A1096",
		"msi": "A648",
	}

	if targetCode, ok := mapping[search]; ok {
		for _, p := range projects {
			if p.ProjectCode == targetCode || p.ID == targetCode {
				return &p, nil
			}
		}
	}

	return nil, fmt.Errorf("project not found for: %s", subCode)
}

func (s *ConfigService) GetDevicesByProjectCode(code string) ([]models.Device, error) {
	devices, err := s.GetAllDevices()
	if err != nil {
		return nil, err
	}

	var result []models.Device
	for _, d := range devices {
		apps := strings.Split(d.AllowedApp, ",")
		for _, app := range apps {
			if strings.TrimSpace(app) == code {
				result = append(result, d)
				break
			}
		}
	}
	return result, nil
}
func (s *ConfigService) SearchProjectsFuzzy(query string) []models.Project {
	projects, err := s.GetAllProjects()
	if err != nil {
		return nil
	}

	search := strings.ToLower(strings.TrimSpace(query))
	var results []models.Project
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.ID), search) ||
			strings.Contains(strings.ToLower(p.ProjectCode), search) ||
			strings.Contains(strings.ToLower(p.ProjectName), search) ||
			strings.Contains(strings.ToLower(p.ShortCode), search) {
			results = append(results, p)
		}
	}
	return results
}
