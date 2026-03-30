package services

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// JungleMessage defines the structure for P2P relay messages
type JungleMessage struct {
	Type    string      `json:"type"`    // match_start, sync_board, action, error, disconnect
	Payload interface{} `json:"payload"` 
}

// JunglePlayer represents a connected player in the lobby or a game
type JunglePlayer struct {
	ID       string
	Nickname string
	Username string
	Conn     *websocket.Conn
	Send     chan JungleMessage
	Room     *JungleRoom
	Deleted  bool       // 标记玩家是否已注销/离线
	Lock     sync.Mutex // 防止竞态
}

// JungleRoom represents a game session between two players
type JungleRoom struct {
	ID      string
	Host    *JunglePlayer
	Guest   *JunglePlayer
	Lock    sync.Mutex
}

var (
	// Matching queue
	waitQueue = make(chan *JunglePlayer, 100)
	// Active rooms
	rooms     = make(map[string]*JungleRoom)
	roomsLock sync.RWMutex
)

// JungleChessWSHandler handles the WebSocket connection for Jungle Chess
func JungleChessWSHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
		return
	}

	user, err := GetUserByID(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[JungleWS] Upgrade error: %v", err)
		return
	}

	player := &JunglePlayer{
		ID:       user.ID,
		Nickname: user.Nickname,
		Username: user.Username,
		Conn:     conn,
		Send:     make(chan JungleMessage, 10),
	}

	log.Printf("[JungleWS] Player %s (%s) connected", player.Nickname, player.ID)

	// Start writer goroutine
	go player.writePump()
	
	// Join matching queue
	select {
	case waitQueue <- player:
		log.Printf("[JungleWS] Player %s joined matching queue", player.Nickname)
	default:
		player.Send <- JungleMessage{Type: "error", Payload: "服务器繁忙，匹配队列已满"}
		conn.Close()
		return
	}

	// Start reader loop
	player.readPump()
}

// writePump handles outgoing messages to the WebSocket
func (p *JunglePlayer) writePump() {
	defer p.Conn.Close()
	for msg := range p.Send {
		if err := p.Conn.WriteJSON(msg); err != nil {
			log.Printf("[JungleWS] Write error for %s: %v", p.Nickname, err)
			return
		}
	}
}

// readPump handles incoming messages and relays them to the opponent
func (p *JunglePlayer) readPump() {
	defer func() {
		p.handleDisconnect()
		p.Conn.Close()
	}()

	for {
		_, message, err := p.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg JungleMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Relay logic
		if p.Room != nil {
			var opponent *JunglePlayer
			if p.Room.Host == p {
				opponent = p.Room.Guest
			} else {
				opponent = p.Room.Host
			}

			if opponent != nil {
				select {
				case opponent.Send <- msg:
				default:
					log.Printf("[JungleWS] Failed to relay message to opponent of %s", p.Nickname)
				}
			}
		}
	}
}

func (p *JunglePlayer) handleDisconnect() {
	p.Lock.Lock()
	p.Deleted = true
	p.Lock.Unlock()

	log.Printf("[JungleWS] Player %s disconnected", p.Nickname)
	if p.Room != nil {
		var opponent *JunglePlayer
		if p.Room.Host == p {
			opponent = p.Room.Guest
		} else {
			opponent = p.Room.Host
		}

		if opponent != nil {
			opponent.Send <- JungleMessage{Type: "disconnect", Payload: "对方已离线"}
			opponent.Room = nil
		}
		
		roomsLock.Lock()
		delete(rooms, p.Room.ID)
		roomsLock.Unlock()
	}
}

// StartMatchmaker runs in background to pair players
func StartMatchmaker() {
	go func() {
		var pending *JunglePlayer
		for {
			p := <-waitQueue

			// 检查玩家是否仍在线
			p.Lock.Lock()
			isDeleted := p.Deleted
			p.Lock.Unlock()

			if isDeleted {
				log.Printf("[JungleWS] Skipping disconnected player %s from queue", p.Nickname)
				continue
			}

			if pending == nil {
				pending = p
				continue
			}

			// 如果 pending 已经离线，则替换为当前玩家
			pending.Lock.Lock()
			pendingDeleted := pending.Deleted
			pending.Lock.Unlock()

			if pendingDeleted {
				pending = p
				continue
			}

			// 成功配对
			p1, p2 := pending, p
			pending = nil

			// 防止同一个账号打开两个窗口自连（可选逻辑）
			if p1.ID == p2.ID {
				p2.Send <- JungleMessage{Type: "error", Payload: "不能与自己对战"}
				p2.Conn.Close()
				continue
			}

			// Basic room setup
			roomID := p1.ID + "_" + p2.ID
			room := &JungleRoom{
				ID:    roomID,
				Host:  p1,
				Guest: p2,
			}

			p1.Room = room
			p2.Room = room

			roomsLock.Lock()
			rooms[roomID] = room
			roomsLock.Unlock()

			// Notify players
			p1.Send <- JungleMessage{
				Type: "match_start",
				Payload: gin.H{
					"role":     "host",
					"opponent": gin.H{"id": p2.ID, "nickname": p2.Nickname, "username": p2.Username},
				},
			}
			p2.Send <- JungleMessage{
				Type: "match_start",
				Payload: gin.H{
					"role":     "guest",
					"opponent": gin.H{"id": p1.ID, "nickname": p1.Nickname, "username": p1.Username},
				},
			}

			log.Printf("[JungleWS] Matched Room %s: %s vs %s", roomID, p1.Nickname, p2.Nickname)
		}
	}()
}
