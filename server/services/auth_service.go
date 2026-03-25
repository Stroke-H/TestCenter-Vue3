package services

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

// --- Models ---

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    string `json:"created_at"`
	FeishuOpenID string `json:"feishu_open_id,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Nickname string `json:"nickname"`
	Password string `json:"password" binding:"required"`
}

// --- Service Logic ---

var (
	userFile = "data/users.jsonl"
	userLock sync.RWMutex
)

// AuthenticateUser verifies credentials by username or email
func AuthenticateUser(identifier, password string) (*User, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	user, err := findUserByIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("用户不存在")
	}

	// Detect hash format
	if strings.HasPrefix(user.PasswordHash, "pbkdf2:") || strings.HasPrefix(user.PasswordHash, "scrypt:") {
		if verifyLegacyHash(user.PasswordHash, password) {
			return user, nil
		}
		return nil, errors.New("密码错误")
	}

	// Default to Bcrypt for new users
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("密码错误")
	}

	return user, nil
}

// RegisterUser hashes password and saves to JSONL
func RegisterUser(username, nickname, password string) (*User, error) {
	userLock.Lock()
	defer userLock.Unlock()

	// Check if user exists
	existing, _ := findUserByIdentifier(username)
	if existing != nil {
		return nil, errors.New("用户已存在")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           uuid.New().String(),
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	// Save to JSONL
	f, err := os.OpenFile(userFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, _ := json.Marshal(user)
	if _, err := f.Write(append(b, '\n')); err != nil {
		return nil, err
	}

	return user, nil
}

func verifyLegacyHash(pwhash, password string) bool {
	parts := strings.Split(pwhash, "$")
	if len(parts) != 3 {
		return false
	}

	algoPart := parts[0] // e.g., "pbkdf2:sha256:260000" or "scrypt:32768:8:1"
	salt := parts[1]
	expectedHashHex := parts[2]

	algoParts := strings.Split(algoPart, ":")
	if len(algoParts) < 2 {
		return false
	}

	method := algoParts[0]

	var actualHash []byte

	if method == "pbkdf2" {
		if len(algoParts) < 3 {
			return false
		}
		iterations := 0
		fmt.Sscanf(algoParts[2], "%d", &iterations)
		if iterations == 0 {
			iterations = 260000
		}
		actualHash = pbkdf2.Key([]byte(password), []byte(salt), iterations, 32, sha256.New)
	} else if method == "scrypt" {
		if len(algoParts) < 4 {
			return false
		}
		var n, r, p int
		fmt.Sscanf(algoParts[1], "%d", &n)
		fmt.Sscanf(algoParts[2], "%d", &r)
		fmt.Sscanf(algoParts[3], "%d", &p)
		
		var err error
		// FIX: Use 64 bytes instead of 32 for legacy scrypt hashes
		actualHash, err = scrypt.Key([]byte(password), []byte(salt), n, r, p, 64)
		if err != nil {
			return false
		}
	} else {
		return false
	}

	actualHashHex := hex.EncodeToString(actualHash)
	return actualHashHex == expectedHashHex
}

func findUserByIdentifier(identifier string) (*User, error) {
	f, err := os.Open(userFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	identifier = strings.ToLower(identifier)
	for scanner.Scan() {
		var user User
		if err := json.Unmarshal(scanner.Bytes(), &user); err == nil {
			if strings.ToLower(user.Username) == identifier || strings.ToLower(user.Email) == identifier {
				return &user, nil
			}
		}
	}
	return nil, nil
}

// GetUserByID retrieves user info
func GetUserByID(id string) (*User, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	f, err := os.Open(userFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var u User
		if err := json.Unmarshal(scanner.Bytes(), &u); err == nil {
			if u.ID == id {
				return &u, nil
			}
		}
	}
	return nil, errors.New("用户不存在")
}

// FindUserByFuzzyName searches for a user and returns if it was an exact match
func FindUserByFuzzyName(name string) (*User, bool, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	f, err := os.Open(userFile)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	var fuzzyMatch *User
	target := strings.ToLower(name)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var u User
		if err := json.Unmarshal(scanner.Bytes(), &u); err == nil {
			uname := strings.ToLower(u.Username)
			uemail := strings.ToLower(u.Email)
			unick := strings.ToLower(u.Nickname)

			// Exact match priority (Username, Email, or Nickname)
			if uname == target || uemail == target || unick == target {
				return &u, true, nil
			}
			// Fuzzy match (contains) as fallback
			if fuzzyMatch == nil && (strings.Contains(uname, target) || strings.Contains(uemail, target) || strings.Contains(unick, target)) {
				fuzzyMatch = &u
			}
		}
	}

	if fuzzyMatch != nil {
		return fuzzyMatch, false, nil
	}
	return nil, false, nil
}

// FindUserByFeishuOpenID checks if an OpenID is already bound
func FindUserByFeishuOpenID(openID string) (*User, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	f, err := os.Open(userFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var u User
		if err := json.Unmarshal(scanner.Bytes(), &u); err == nil {
			if u.FeishuOpenID == openID {
				return &u, nil
			}
		}
	}
	return nil, nil
}

// UpdateUserFeishuOpenID updates a user's Feishu binding
func UpdateUserFeishuOpenID(userID, openID string) error {
	userLock.Lock()
	defer userLock.Unlock()

	f, err := os.Open(userFile)
	if err != nil {
		return err
	}

	var users []User
	scanner := bufio.NewScanner(f)
	updated := false
	for scanner.Scan() {
		var u User
		if err := json.Unmarshal(scanner.Bytes(), &u); err == nil {
			if u.ID == userID {
				u.FeishuOpenID = openID
				updated = true
			}
			users = append(users, u)
		}
	}
	f.Close()

	if !updated {
		return errors.New("用户未找到，无法绑定")
	}

	// Rewrite file
	wf, err := os.OpenFile(userFile, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer wf.Close()

	for _, u := range users {
		b, _ := json.Marshal(u)
		wf.Write(append(b, '\n'))
	}

	return nil
}

// UnbindFeishuOpenID removes a Feishu binding by OpenID
func UnbindFeishuOpenID(openID string) error {
	userLock.Lock()
	defer userLock.Unlock()

	f, err := os.Open(userFile)
	if err != nil {
		return err
	}

	var users []User
	scanner := bufio.NewScanner(f)
	updated := false
	for scanner.Scan() {
		var u User
		if err := json.Unmarshal(scanner.Bytes(), &u); err == nil {
			if u.FeishuOpenID == openID {
				u.FeishuOpenID = ""
				updated = true
			}
			users = append(users, u)
		}
	}
	f.Close()

	if !updated {
		return errors.New("当前飞书用户尚未绑定任何测试账户")
	}

	// Rewrite file
	wf, err := os.OpenFile(userFile, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer wf.Close()

	for _, u := range users {
		b, _ := json.Marshal(u)
		wf.Write(append(b, '\n'))
	}

	return nil
}

// --- Handlers ---

func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	user, err := RegisterUser(req.Username, req.Nickname, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user": gin.H{
			"username": user.Username,
			"nickname": user.Nickname,
		},
	})
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	user, err := AuthenticateUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   user.ID,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"nickname": user.Nickname,
		},
	})
}

func GetUserMeHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := GetUserByID(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
	})
}
