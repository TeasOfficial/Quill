package onebot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quill/internal/logc"

	"github.com/gorilla/websocket"
)

type Receiver struct {
	Conn      *websocket.Conn
	OnEvent   func(*Event)
	OnConnect func()
}

func NewReceiver() *Receiver {
	return &Receiver{}
}

func (r *Receiver) Connect(baseURL, token, wsPath string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = wsPath

	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial %s: HTTP %d - %w", u.String(), resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial %s: %w", u.String(), err)
	}
	r.Conn = conn
	logc.Fmt("\u2713", logc.Green, "WS \u5df2\u8fde\u63a5 %s", u.Host)

	if r.OnConnect != nil {
		r.OnConnect()
	}
	return nil
}

func (r *Receiver) Serve(addr, path, token string) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if token != "" {
			auth := req.Header.Get("Authorization")
			if auth != "Bearer "+token && req.URL.Query().Get("access_token") != token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		if req.Header.Get("Upgrade") == "websocket" {
			conn, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				log.Printf("[WS] Upgrade error: %v", err)
				return
			}
			r.Conn = conn
			logc.Fmt("\u2713", logc.Green, "WS \u5ba2\u6237\u7aef\u8fde\u63a5 %s", req.RemoteAddr)
			if r.OnConnect != nil {
				r.OnConnect()
			}
			r.Listen()
			return
		}

		if req.Method == "POST" {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				log.Printf("[HTTP] Read error: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			req.Body.Close()

			var event Event
			if err := json.Unmarshal(body, &event); err != nil {
				log.Printf("[HTTP] Unmarshal error: %v", err)
				http.Error(w, "Bad JSON", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))

			if r.OnEvent != nil {
				r.OnEvent(&event)
			}
		}
	})

	logc.Fmt("\u2637", logc.Gray, "\u76d1\u542c %s%s", addr, path)
	return http.ListenAndServe(addr, mux)
}

func (r *Receiver) Listen() {
	defer r.Conn.Close()
	for {
		_, msg, err := r.Conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "close") {
				log.Println("[WS] Connection closed")
				return
			}
			log.Printf("[WS] Read error: %v", err)
			return
		}

		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Printf("[WS] Failed to unmarshal event: %v", err)
			continue
		}

		if r.OnEvent != nil {
			r.OnEvent(&event)
		}
	}
}
