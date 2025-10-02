package main

// Will contain HTTP server logic and handlers for primary

import (
	"encoding/json"
	"net/http"
)

// HTTP request for API calls
type HTTPRequest struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

// Internal request for actor ops
type Request struct {
	Key string
	Val string
}

func (a *Actor) writeHandler(w http.ResponseWriter, r *http.Request) {
	var req HTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.writeFromClient(Request{Key: req.Key, Val: req.Val}) // Call actor method to handle write (Create LSN, log, send to backups, commit) - Do this in actors.go

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ack"))
}

func (a *Actor) startServer() {
	http.HandleFunc("/write/{key}/{value}", a.writeHandler)
	http.ListenAndServe(":8080", nil)
}
