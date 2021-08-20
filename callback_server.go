package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func spawnCbkServer(addr string) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleCallback)

	srv := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 13,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("srv.ListenAndServe() failed: %v", err)
		}
	}()

	return srv, nil
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Printf("r.ParseForm() failed: %v", err)
		return
	}

	var resp FrontendResponse
	_ = decoder.Decode(&resp, r.PostForm)

	if resp.MessageUUID == "" || resp.MessageState == "" {
		return
	}

	switch resp.MessageState {
	case "sent", "failed", "delivered", "undelivered":
		l := strings.Split(r.URL.Path, "/")
		reqID := l[len(l)-1]
		if entry, ok := store.Get(reqID); ok {
			select {
			case entry.CbkCh <- resp:
			default:
			}
		}
	case "queued":
	default:
	}
}
