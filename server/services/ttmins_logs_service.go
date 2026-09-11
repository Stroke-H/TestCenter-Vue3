package services

import (
	"crypto/rand"
	"crypto/sha512"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const ttminsLogsBasePath = "/ttmins-logs/"

//go:embed assets/ttmins_logs/target.js assets/ttmins_logs/integrity.json assets/ttmins_logs/CHII-LICENSE assets/ttmins_logs/front_end
var ttminsLogsAssets embed.FS

var ttminsLogsIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type ttminsLogTarget struct {
	ID          string
	URL         string
	Title       string
	IP          string
	UserAgent   string
	ConnectedAt time.Time
	Target      *websocket.Conn
	Client      *websocket.Conn
	Ticket      string
	TicketUntil time.Time
	targetWrite sync.Mutex
}

type ttminsLogsHub struct {
	mu        sync.RWMutex
	targets   map[string]*ttminsLogTarget
	timestamp int64
}

var globalTTminsLogsHub = &ttminsLogsHub{targets: map[string]*ttminsLogTarget{}, timestamp: time.Now().UnixMilli()}

var ttminsLogsUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	// The controlled script is intentionally loaded into a different origin.
	CheckOrigin: func(*http.Request) bool { return true },
}

func RegisterTTminsLogsRoutes(router *gin.Engine, api *gin.RouterGroup) error {
	if err := validateTTminsLogsClientAssets(); err != nil {
		return err
	}
	staticAssets, err := fs.Sub(ttminsLogsAssets, "assets/ttmins_logs")
	if err != nil {
		return err
	}
	staticHandler := http.StripPrefix(ttminsLogsBasePath, http.FileServer(http.FS(staticAssets)))
	router.GET(ttminsLogsBasePath+"target.js", ttminsLogsAssetHeaders, gin.WrapH(staticHandler))
	router.HEAD(ttminsLogsBasePath+"target.js", ttminsLogsAssetHeaders, gin.WrapH(staticHandler))
	router.GET(ttminsLogsBasePath+"integrity.json", ttminsLogsAssetHeaders, gin.WrapH(staticHandler))
	router.GET(ttminsLogsBasePath+"front_end/*filepath", ttminsLogsAssetHeaders, gin.WrapH(staticHandler))
	router.GET(ttminsLogsBasePath+"target/:id", TTminsLogsTargetWebSocketHandler)
	router.GET(ttminsLogsBasePath+"client/:id", TTminsLogsClientWebSocketHandler)
	router.GET(ttminsLogsBasePath+"proxy", TTminsLogsProxyHandler)

	group := api.Group("/ttmins-logs")
	group.Use(func(c *gin.Context) {
		if _, err := CurrentUserFromRequest(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
			c.Abort()
			return
		}
		c.Next()
	})
	group.GET("/info", GetTTminsLogsInfoHandler)
	group.GET("/targets", ListTTminsLogTargetsHandler)
	group.POST("/targets/:id/inspect", CreateTTminsLogInspectHandler)
	group.DELETE("/targets/:id", DisconnectTTminsLogTargetHandler)
	return nil
}

func ttminsLogsAssetHeaders(c *gin.Context) {
	if strings.HasSuffix(c.Request.URL.Path, "/target.js") || strings.HasSuffix(c.Request.URL.Path, "/integrity.json") {
		c.Header("Cache-Control", "no-store")
	} else {
		c.Header("Cache-Control", "public, max-age=7200")
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
}

func TTminsLogsTargetWebSocketHandler(c *gin.Context) {
	id := c.Param("id")
	if !ttminsLogsIDPattern.MatchString(id) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	targetURL := limitedQuery(c.Query("url"), 2048)
	if parsed, err := url.ParseRequestURI(targetURL); err != nil || parsed.Scheme == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	connection, err := ttminsLogsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	target := &ttminsLogTarget{
		ID: id, URL: targetURL, Title: limitedQuery(c.Query("title"), 256), IP: remoteClientIP(c.Request),
		UserAgent: limitedQuery(c.GetHeader("User-Agent"), 512), ConnectedAt: time.Now(), Target: connection,
	}
	globalTTminsLogsHub.attachTarget(target)
	defer globalTTminsLogsHub.removeTarget(target)
	for {
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			return
		}
		client := globalTTminsLogsHub.clientFor(target)
		if client != nil && client.WriteMessage(messageType, payload) != nil {
			globalTTminsLogsHub.detachClient(target, client)
		}
	}
}

func TTminsLogsClientWebSocketHandler(c *gin.Context) {
	id := c.Param("id")
	targetID, ticket := c.Query("target"), c.Query("ticket")
	if !ttminsLogsIDPattern.MatchString(id) || !ttminsLogsIDPattern.MatchString(targetID) || ticket == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	target := globalTTminsLogsHub.consumeTicket(targetID, ticket)
	if target == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	connection, err := ttminsLogsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	globalTTminsLogsHub.attachClient(target, connection)
	defer globalTTminsLogsHub.detachClient(target, connection)
	for {
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			return
		}
		target.targetWrite.Lock()
		err = target.Target.WriteMessage(messageType, payload)
		target.targetWrite.Unlock()
		if err != nil {
			return
		}
	}
}

func (hub *ttminsLogsHub) attachTarget(target *ttminsLogTarget) {
	hub.mu.Lock()
	old := hub.targets[target.ID]
	hub.targets[target.ID] = target
	hub.timestamp = time.Now().UnixMilli()
	hub.mu.Unlock()
	if old != nil {
		old.Target.Close()
		if old.Client != nil {
			old.Client.Close()
		}
	}
}

func (hub *ttminsLogsHub) removeTarget(target *ttminsLogTarget) {
	hub.mu.Lock()
	if hub.targets[target.ID] != target {
		hub.mu.Unlock()
		return
	}
	delete(hub.targets, target.ID)
	hub.timestamp = time.Now().UnixMilli()
	client := target.Client
	hub.mu.Unlock()
	if client != nil {
		client.Close()
	}
}

func (hub *ttminsLogsHub) clientFor(target *ttminsLogTarget) *websocket.Conn {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if hub.targets[target.ID] != target {
		return nil
	}
	return target.Client
}

func (hub *ttminsLogsHub) attachClient(target *ttminsLogTarget, client *websocket.Conn) {
	hub.mu.Lock()
	old := target.Client
	target.Client = client
	hub.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (hub *ttminsLogsHub) detachClient(target *ttminsLogTarget, client *websocket.Conn) {
	hub.mu.Lock()
	if target.Client == client {
		target.Client = nil
	}
	hub.mu.Unlock()
	client.Close()
}

func (hub *ttminsLogsHub) consumeTicket(targetID, ticket string) *ttminsLogTarget {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	target := hub.targets[targetID]
	if target == nil || target.Ticket != ticket || time.Now().After(target.TicketUntil) {
		return nil
	}
	target.Ticket = ""
	target.TicketUntil = time.Time{}
	return target
}

func GetTTminsLogsInfoHandler(c *gin.Context) {
	manifest, err := fs.ReadFile(ttminsLogsAssets, "assets/ttmins_logs/integrity.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "日志客户端信息不可用"})
		return
	}
	var info struct {
		Version   string `json:"version"`
		Integrity string `json:"integrity"`
	}
	if json.Unmarshal(manifest, &info) != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "日志客户端信息无效"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"version": info.Version, "integrity": info.Integrity,
		"script_path": ttminsLogsBasePath + "target.js", "script_options": ttminsLogsScriptOptions(),
	})
}

type ttminsLogsScriptOption struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func ttminsLogsScriptOptions() []ttminsLogsScriptOption {
	port := "5173"
	configuredOrigin := strings.TrimSpace(os.Getenv("TESTCENTER_FRONTEND_ORIGIN"))
	if parsed, err := url.Parse(configuredOrigin); err == nil && parsed.Port() != "" {
		port = parsed.Port()
	}

	options := make([]ttminsLogsScriptOption, 0, 4)
	seen := map[string]bool{}
	for _, item := range ttminsLogsLANAddresses() {
		scriptURL := "http://" + net.JoinHostPort(item.IP, port) + ttminsLogsBasePath + "target.js"
		if !seen[scriptURL] {
			seen[scriptURL] = true
			options = append(options, ttminsLogsScriptOption{Label: "局域网 · " + item.IP, URL: scriptURL})
		}
	}
	if parsed, err := url.Parse(configuredOrigin); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		scriptURL := strings.TrimRight(configuredOrigin, "/") + ttminsLogsBasePath + "target.js"
		if !seen[scriptURL] {
			options = append(options, ttminsLogsScriptOption{Label: "配置域名 · " + parsed.Host, URL: scriptURL})
		}
	}
	return options
}

type ttminsLogsLANAddress struct {
	IP        string
	Interface string
}

func ttminsLogsLANAddresses() []ttminsLogsLANAddress {
	interfaces, _ := net.Interfaces()
	addresses := make([]ttminsLogsLANAddress, 0, len(interfaces))
	for _, networkInterface := range interfaces {
		name := strings.ToLower(networkInterface.Name)
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 ||
			strings.HasPrefix(name, "utun") || strings.HasPrefix(name, "bridge") ||
			strings.HasPrefix(name, "awdl") || strings.HasPrefix(name, "llw") {
			continue
		}
		interfaceAddresses, _ := networkInterface.Addrs()
		for _, address := range interfaceAddresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || !ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			addresses = append(addresses, ttminsLogsLANAddress{IP: ip.String(), Interface: networkInterface.Name})
		}
	}
	sort.SliceStable(addresses, func(i, j int) bool {
		leftPreferred := addresses[i].Interface == "en0" || addresses[i].Interface == "en1"
		rightPreferred := addresses[j].Interface == "en0" || addresses[j].Interface == "en1"
		if leftPreferred != rightPreferred {
			return leftPreferred
		}
		return addresses[i].Interface < addresses[j].Interface
	})
	return addresses
}

func ListTTminsLogTargetsHandler(c *gin.Context) {
	globalTTminsLogsHub.mu.RLock()
	targets := make([]gin.H, 0, len(globalTTminsLogsHub.targets))
	for _, target := range globalTTminsLogsHub.targets {
		targets = append(targets, gin.H{
			"id": target.ID, "title": target.Title, "url": target.URL, "ip": target.IP,
			"user_agent": target.UserAgent, "connected_at": target.ConnectedAt, "inspecting": target.Client != nil,
		})
	}
	timestamp := globalTTminsLogsHub.timestamp
	globalTTminsLogsHub.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"targets": targets, "timestamp": timestamp})
}

func CreateTTminsLogInspectHandler(c *gin.Context) {
	targetID := c.Param("id")
	if !ttminsLogsIDPattern.MatchString(targetID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "设备标识无效"})
		return
	}
	ticketBytes := make([]byte, 32)
	if _, err := rand.Read(ticketBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建检查会话"})
		return
	}
	ticket := base64.RawURLEncoding.EncodeToString(ticketBytes)
	clientIDBytes := make([]byte, 12)
	if _, err := rand.Read(clientIDBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建检查会话"})
		return
	}
	clientID := base64.RawURLEncoding.EncodeToString(clientIDBytes)
	globalTTminsLogsHub.mu.Lock()
	target := globalTTminsLogsHub.targets[targetID]
	if target != nil {
		target.Ticket, target.TicketUntil = ticket, time.Now().Add(2*time.Minute)
	}
	globalTTminsLogsHub.mu.Unlock()
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备已离线"})
		return
	}
	clientPath := ttminsLogsBasePath + "client/" + clientID + "?target=" + url.QueryEscape(targetID) + "&ticket=" + url.QueryEscape(ticket)
	c.JSON(http.StatusOK, gin.H{"client_path": clientPath, "inspector_path": ttminsLogsBasePath + "front_end/chii_app.html"})
}

func DisconnectTTminsLogTargetHandler(c *gin.Context) {
	globalTTminsLogsHub.mu.RLock()
	target := globalTTminsLogsHub.targets[c.Param("id")]
	globalTTminsLogsHub.mu.RUnlock()
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备已离线"})
		return
	}
	target.Target.Close()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func TTminsLogsProxyHandler(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return
	}
	destination, err := url.Parse(c.Query("url"))
	if err != nil || (destination.Scheme != "http" && destination.Scheme != "https") || destination.User != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !globalTTminsLogsHub.proxyOriginAllowed(destination) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if !globalTTminsLogsHub.proxyOriginAllowed(req.URL) {
			return errors.New("redirect origin is not an active target")
		}
		return nil
	}}
	request, _ := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, destination.String(), nil)
	request.Header.Set("User-Agent", limitedQuery(c.GetHeader("User-Agent"), 512))
	response, err := client.Do(request)
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "Last-Modified", "ETag"} {
		if value := response.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
	c.Header("Access-Control-Allow-Origin", "*")
	c.Status(response.StatusCode)
	io.Copy(c.Writer, io.LimitReader(response.Body, 20<<20))
}

func (hub *ttminsLogsHub) proxyOriginAllowed(destination *url.URL) bool {
	destinationHost := strings.ToLower(destination.Host)
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for _, target := range hub.targets {
		parsed, err := url.Parse(target.URL)
		if err == nil && strings.EqualFold(parsed.Scheme, destination.Scheme) && strings.ToLower(parsed.Host) == destinationHost {
			return true
		}
	}
	return false
}

func limitedQuery(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}

func validateTTminsLogsClientAssets() error {
	target, err := fs.ReadFile(ttminsLogsAssets, "assets/ttmins_logs/target.js")
	if err != nil {
		return err
	}
	manifestContent, err := fs.ReadFile(ttminsLogsAssets, "assets/ttmins_logs/integrity.json")
	if err != nil {
		return err
	}
	var manifest struct {
		ProtocolVersion int    `json:"protocolVersion"`
		Version         string `json:"version"`
		Integrity       string `json:"integrity"`
	}
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		return err
	}
	digest := sha512.Sum384(target)
	integrity := "sha384-" + base64.StdEncoding.EncodeToString(digest[:])
	if manifest.ProtocolVersion != 1 || strings.TrimSpace(manifest.Version) == "" || manifest.Integrity != integrity {
		return errors.New("controlled TTmins log client integrity mismatch")
	}
	return nil
}

func remoteClientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func requestScheme(request *http.Request) string {
	if strings.EqualFold(strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		return "https"
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}
