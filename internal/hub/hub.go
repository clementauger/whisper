package hub

import (
	"crypto/rand"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/knadh/niltalk/internal/notify"
)

// Types of messages sent to peers.
const (
	// TypeTyping = "typing"
	// TypeMessage         = "message"
	// TypeBroadcast       = "broadcast"
	TypeUploading = "uploading"
	TypeUpload    = "upload"
	TypePeerList  = "peer.list"
	// TypePeerInfo        = "peer.info"
	TypePeerJoin        = "peer.join"
	TypePeerLeave       = "peer.leave"
	TypePeerRateLimited = "peer.ratelimited"
	TypeRoomDispose     = "room.dispose"
	TypeRoomFull        = "room.full"
	TypeNotice          = "notice"
	// TypeHandle          = "handle"
	TypeGrowl = "growl"
	// TypePing            = "ping"
	// TypeWhisper         = "whisper"
	TypeMotd = "motd"
	// TypeChallengeQuery  = "challenge.query"
	// TypeChallengeRes    = "challenge.res"
)

// Config represents the app configuration.
type Config struct {
	Address string `koanf:"address"`
	RootURL string `koanf:"root_url"`

	Name              string        `koanf:"name"`
	RoomIDLen         int           `koanf:"room_id_length"`
	MaxCachedMessages int           `koanf:"max_cached_messages"`
	MaxMessageLen     int           `koanf:"max_message_length"`
	WSTimeout         time.Duration `koanf:"websocket_timeout"`
	MaxMessageQueue   int           `koanf:"max_message_queue"`
	RateLimitInterval time.Duration `koanf:"rate_limit_interval"`
	RateLimitMessages int           `koanf:"rate_limit_messages"`
	MaxRooms          int           `koanf:"max_rooms"`
	MaxPeersPerRoom   int           `koanf:"max_peers_per_room"`
	PeerHandleFormat  string        `koanf:"peer_handle_format"`
	RoomTimeout       time.Duration `koanf:"room_timeout"`
	RoomAge           time.Duration `koanf:"room_age"`
	SessionCookie     string        `koanf:"session_cookie"`
	Storage           string        `koanf:"storage"`

	Rooms map[string]PredefinedRoom `koanf:"rooms"`

	Theme string `koanf:"theme"`
}

// PredefinedRoom are static rooms declared in the configuration file.
type PredefinedRoom struct {
	ID       string           `koanf:"id"`
	Name     string           `koanf:"name"`
	Password string           `koanf:"password"`
	Growl    notify.Options   `koanf:"growl"`
	Users    []PredefinedUser `koanf:"users"`
	Motd     string           `koanf:"motd"`
}

// PredefinedUser are static users declared in the configuration file.
type PredefinedUser struct {
	Name     string `koanf:"name"`
	Password string `koanf:"password"`
	Growl    bool   `koanf:"growl"`
}

// Hub acts as the controller and container for all chat rooms.
type Hub struct {
	rooms map[string]*Room

	cfg *Config
	mut sync.RWMutex
	log *log.Logger
}

// NewHub returns a new instance of Hub.
func NewHub(cfg *Config, l *log.Logger) *Hub {
	return &Hub{
		rooms: make(map[string]*Room),

		cfg: cfg,
		log: l,
	}
}

// AddRoom creates a new room in the store, adds it to the hub, and
// returns the room (which has to be .Run() on a goroutine then).
func (h *Hub) AddRoom(name string) (*Room, error) {

	id, err := h.generateRoomID(h.cfg.RoomIDLen, 5)
	if err != nil {
		return nil, err
	}

	// Initialize the room.
	return h.initRoom(id, name, false), nil
}

// AddPredefinedRoom creates a predefined room in the store, adds it to the hub.
// If it already exists, no error is returned.
func (h *Hub) AddPredefinedRoom(ID, name string) (*Room, error) {

	// Initialize the room.
	return h.initRoom(ID, name, true), nil
}

// GetRoom retrives an active room from the hub.
func (h *Hub) GetRoom(id string) *Room {
	h.mut.Lock()
	r, _ := h.rooms[id]
	h.mut.Unlock()
	return r
}

// initRoom initializes a room on the Hub.
func (h *Hub) initRoom(id, name string, predefined bool) *Room {
	r := NewRoom(id, name, h, predefined)
	h.mut.Lock()
	defer h.mut.Unlock()
	if predefined {
		r.motd = h.cfg.Rooms[id].Motd
	}
	h.rooms[id] = r
	go r.run()
	return r
}

// getRooms returns the list of active rooms.
func (h *Hub) getRooms() []*Room {
	h.mut.RLock()
	defer h.mut.RUnlock()
	out := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		out = append(out, r)
	}
	return out
}

// removeRoom removes a room from the hub and the store.
func (h *Hub) removeRoom(id string) error {
	h.mut.Lock()
	defer h.mut.Unlock()
	delete(h.rooms, id)
	return nil
}

// generateRoomID generates a random room ID while checking the store for
// uniqueness up to numTries times.
func (h *Hub) generateRoomID(length, numTries int) (string, error) {
	for i := 0; i < numTries; i++ {
		id, err := GenerateGUID(length)
		if err != nil {
			h.log.Printf("error generating room ID: %v", err)
			return "", errors.New("error generating room ID")
		}

		_, exists := h.rooms[id]
		if !exists {
			return id, nil
		}
	}
	return "", errors.New("unable to generate unique room ID")
}

// GenerateGUID generates a cryptographically random, alphanumeric string of length n.
func GenerateGUID(n int) (string, error) {
	const dictionary = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return string(bytes), nil
}
