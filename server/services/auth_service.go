package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
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
	Avatar       string `json:"avatar,omitempty"`
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

type AccountProfile struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Email        string `json:"email"`
	Avatar       string `json:"avatar,omitempty"`
	CreatedAt    string `json:"created_at"`
	FeishuOpenID string `json:"feishu_open_id,omitempty"`
}

type AccountUpdateRequest struct {
	ID       string `json:"id" binding:"required"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

type CurrentProfileUpdateRequest struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

// --- Service Logic ---

var (
	userLock sync.RWMutex
)

var legacyUserIDAliases = map[string]string{
	"3": "minghong",
	"4": "rongchang.xing",
}

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

// RegisterUser hashes password and saves to the SQL-backed testers store.
func RegisterUser(username, nickname, password string) (*User, error) {
	userLock.Lock()
	defer userLock.Unlock()

	if err := ensureTestersAvatarColumn(); err != nil {
		return nil, err
	}

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

	if err := sqlUpsertJSON("testers", user); err != nil {
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
	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	identifier = strings.ToLower(identifier)
	for i := range users {
		user := &users[i]
		if strings.ToLower(user.Username) == identifier || strings.ToLower(user.Email) == identifier {
			return user, nil
		}
	}
	return nil, nil
}

func findUserByUsername(username string) (*User, error) {
	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	target := strings.ToLower(strings.TrimSpace(username))
	for i := range users {
		if strings.ToLower(strings.TrimSpace(users[i].Username)) == target {
			return &users[i], nil
		}
	}
	return nil, nil
}

// GetUserByID retrieves user info
func GetUserByID(id string) (*User, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == id {
			return &users[i], nil
		}
	}

	if username, ok := legacyUserIDAliases[strings.TrimSpace(id)]; ok {
		for i := range users {
			if strings.EqualFold(strings.TrimSpace(users[i].Username), username) {
				return &users[i], nil
			}
		}
	}
	return nil, errors.New("用户不存在")
}

// ListAccountProfiles returns all platform accounts without exposing password hashes.
func ListAccountProfiles() ([]AccountProfile, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	profiles := make([]AccountProfile, 0)
	for _, user := range users {
		profiles = append(profiles, AccountProfile{
			ID:           user.ID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			Email:        user.Email,
			Avatar:       user.Avatar,
			CreatedAt:    user.CreatedAt,
			FeishuOpenID: user.FeishuOpenID,
		})
	}
	return profiles, nil
}

// UpdateAccountProfile updates editable account fields while preserving login credentials.
func UpdateAccountProfile(req AccountUpdateRequest) (*AccountProfile, error) {
	return updateAccountProfileByID(req.ID, req.Nickname, req.Email, req.Avatar)
}

func updateAccountProfileByID(userID, nickname, email, avatar string) (*AccountProfile, error) {
	userLock.Lock()
	defer userLock.Unlock()

	if err := ensureTestersAvatarColumn(); err != nil {
		return nil, err
	}

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	updatedIndex := -1
	for i := range users {
		if users[i].ID == userID {
			users[i].Nickname = nickname
			users[i].Email = email
			users[i].Avatar = avatar
			updatedIndex = i
			break
		}
	}
	if updatedIndex < 0 {
		return nil, errors.New("用户不存在")
	}
	if err := sqlUpsertJSON("testers", users[updatedIndex]); err != nil {
		return nil, err
	}

	updatedUser := users[updatedIndex]
	return &AccountProfile{
		ID:           updatedUser.ID,
		Username:     updatedUser.Username,
		Nickname:     updatedUser.Nickname,
		Email:        updatedUser.Email,
		Avatar:       updatedUser.Avatar,
		CreatedAt:    updatedUser.CreatedAt,
		FeishuOpenID: updatedUser.FeishuOpenID,
	}, nil
}

// FindUserByFuzzyName searches for a user and returns if it was an exact match
func FindUserByFuzzyName(name string) (*User, bool, error) {
	userLock.RLock()
	defer userLock.RUnlock()

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, false, err
	}
	var fuzzyMatch *User
	target := strings.ToLower(name)
	for i := range users {
		u := &users[i]
		uname := strings.ToLower(u.Username)
		uemail := strings.ToLower(u.Email)
		unick := strings.ToLower(u.Nickname)
		if uname == target || uemail == target || unick == target {
			return u, true, nil
		}
		if fuzzyMatch == nil && (strings.Contains(uname, target) || strings.Contains(uemail, target) || strings.Contains(unick, target)) {
			fuzzyMatch = u
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

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].FeishuOpenID == openID {
			return &users[i], nil
		}
	}
	return nil, nil
}

// UpdateUserFeishuOpenID updates a user's Feishu binding
func UpdateUserFeishuOpenID(userID, openID string) error {
	userLock.Lock()
	defer userLock.Unlock()

	if err := ensureTestersAvatarColumn(); err != nil {
		return err
	}

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return err
	}
	updated := false
	var updatedUser User
	for i := range users {
		if users[i].ID == userID {
			users[i].FeishuOpenID = openID
			updatedUser = users[i]
			updated = true
			break
		}
	}
	if !updated {
		return errors.New("用户未找到，无法绑定")
	}
	return sqlUpsertJSON("testers", updatedUser)
}

// UnbindFeishuOpenID removes a Feishu binding by OpenID
func UnbindFeishuOpenID(openID string) error {
	userLock.Lock()
	defer userLock.Unlock()

	if err := ensureTestersAvatarColumn(); err != nil {
		return err
	}

	users, err := sqlListJSON[User]("testers", "`migrated_at` ASC")
	if err != nil {
		return err
	}
	updated := false
	var updatedUser User
	for i := range users {
		if users[i].FeishuOpenID == openID {
			users[i].FeishuOpenID = ""
			updatedUser = users[i]
			updated = true
			break
		}
	}
	if !updated {
		return errors.New("当前飞书用户尚未绑定任何测试账户")
	}
	return sqlUpsertJSON("testers", updatedUser)
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
	if err := createUserSession(c, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"nickname": user.Nickname,
			"email":    user.Email,
			"avatar":   user.Avatar,
		},
	})
}

func GetUserMeHandler(c *gin.Context) {
	user, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
		"email":    user.Email,
		"avatar":   user.Avatar,
	})
}

func UpdateCurrentUserProfileHandler(c *gin.Context) {
	currentUser, err := currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	var req CurrentProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(req.Email)
	req.Avatar = strings.TrimSpace(req.Avatar)

	if req.Nickname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "昵称不能为空"})
		return
	}

	profile, err := updateAccountProfileByID(currentUser.ID, req.Nickname, req.Email, req.Avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "个人设置已更新",
		"user": gin.H{
			"id":       profile.ID,
			"username": profile.Username,
			"nickname": profile.Nickname,
			"email":    profile.Email,
			"avatar":   profile.Avatar,
		},
	})
}

func LogoutHandler(c *gin.Context) {
	revokeCurrentSession(c)
	c.JSON(http.StatusOK, gin.H{"message": "退出成功"})
}

func ensureTestersAvatarColumn() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}

	var columnName string
	err = db.QueryRowContext(ctx, `
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'testers'
			AND COLUMN_NAME = 'avatar'
		LIMIT 1
	`).Scan(&columnName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = db.ExecContext(ctx, "ALTER TABLE testers ADD COLUMN avatar LONGTEXT NULL AFTER email")
	return err
}
