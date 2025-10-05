package server

import (
	"log"
	"net/http"
	"sync"

	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/polling"
	"github.com/googollee/go-socket.io/engineio/transport/websocket"
	"github.com/wizardsord/thai-smartcard-reader/pkg/model"
)

// Easier to get running with CORS.
var allowOriginFunc = func(r *http.Request) bool {
	return true
}

// store last data in memory
var lastCardData any
var mu sync.RWMutex

type socketIO struct {
	*socketio.Server
}

func NewSocketIO() *socketIO {
	server := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			&polling.Transport{
				CheckOrigin: allowOriginFunc,
			},
			&websocket.Transport{
				CheckOrigin: allowOriginFunc,
			},
		},
	})
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		log.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("closed", reason)
	})

	// 🟡 Manual read request — resend last data
    server.OnEvent("/", "readCard", func(s socketio.Conn) {
        mu.RLock()
        data := lastCardData
        mu.RUnlock()

        if data != nil {
            log.Println("🟢 Sending last known card data to", s.ID())
            s.Emit("smc-data", data)
        } else {
            log.Println("⚠️ No previous card data to send")
            s.Emit("smc-error", "ยังไม่มีข้อมูลจากบัตร กรุณาเสียบใหม่")
        }
    })

	return &socketIO{server}
}

func (s *socketIO) Broadcast(msg model.Message) {
    if msg.Event == "smc-data" {
        mu.Lock()
        lastCardData = msg.Payload // remember last good data
        mu.Unlock()
    }
    s.BroadcastToNamespace("/", msg.Event, msg.Payload)
}