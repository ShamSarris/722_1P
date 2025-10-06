package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPRequest represents the JSON body for POST requests
type HTTPRequest struct {
	Val string `json:"val"`
}

// HTTPResponse represents the response to clients
type HTTPResponse struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// PendingRequest tracks requests waiting for quorum
type PendingRequest struct {
	request   *Request
	acks      int
	respChan  chan *Response
	startTime time.Time
	committed bool
}

// Request represents an internal operation
type Request struct {
	// Type string // "READ" or "WRITE"
	Key string
	Val string
	LSN int64
}

// Response represents the result of an operation
type Response struct {
	Success bool
	Key     string
	Value   string
	Error   string
}

// Server manages HTTP endpoints and pending requests
type Server struct {
	actor         *Actor
	pendingReqs   map[int64]*PendingRequest
	committedReqs []int64
	pendingMu     sync.Mutex
	port          int
}

func NewServer(actor *Actor, port int) *Server {
	return &Server{
		actor:       actor,
		pendingReqs: make(map[int64]*PendingRequest),
		port:        port,
	}
}

// Start initializes and starts the HTTP server
func (s *Server) Start() {
	// Setup routes
	http.HandleFunc("/", s.routeHandler)

	// Start server
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting HTTP server on %s", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
}

// routeHandler handles incoming HTTP requests and routes them appropriately
func (s *Server) routeHandler(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL (remove leading slash)
	path := r.URL.Path
	if path == "/" || path == "" {
		s.sendError(w, "Key is required in URL path", http.StatusBadRequest)
		return
	}

	// Remove leading slash
	if path[0] == '/' {
		path = path[1:]
	}

	// Route based on HTTP method
	switch r.Method {
	case http.MethodGet:
		// GET: /key
		log.Printf("Received GET request with path: %s", path)
		s.handleRead(w, r, path)
	case http.MethodPost:
		// POST: /key/value
		log.Printf("Received POST request with path: %s", path)
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			s.sendError(w, "POST requests require format: /key/value", http.StatusBadRequest)
			return
		}
		key := parts[0]
		value := strings.Join(parts[1:], "/") // In case value contains slashes
		s.handleWrite(w, r, key, value)
	default:
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRead processes GET requests for reading keys
func (s *Server) handleRead(w http.ResponseWriter, r *http.Request, key string) {
	log.Printf("Handling READ request for key: %s", key)

	if s.actor.isPrimary {
		// Primary: replicate read and wait for quorum
		// Write actor methods to read and respond

		//resp := s.actor.executeRead(key)
		//s.sendResponse(w, resp)
	} else {
		// Backup: read directly from store
		// Write actor methods to read and respond

		//resp := s.actor.readFromStore(key)
		//s.sendResponse(w, resp)
	}
}

// handleWrite processes POST requests for writing keys
func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request, key string, val string) {
	log.Printf("Handling WRITE request for key: %s, value: %s", key, val)

	// Only primary can accept writes
	if !s.actor.isPrimary {
		s.sendError(w, "Only primary can accept writes", http.StatusForbidden)
		return
	}

	if val == "" {
		s.sendError(w, "Value is required", http.StatusBadRequest)
		return
	}

	// Create request and get response channel
	req := &Request{Key: key, Val: val, LSN: -1}
	respChan := s.RegisterPendingRequest(-1, req) // Will be updated with actual LSN in write()

	// Execute write through actor
	go s.actor.write(req)

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		s.sendResponse(w, resp)
	case <-time.After(30 * time.Second):
		s.sendError(w, "Request timeout", http.StatusRequestTimeout)
	}
}

// sendResponse sends a successful response to the client
func (s *Server) sendResponse(w http.ResponseWriter, resp *Response) {
	if !resp.Success {
		s.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	httpResp := HTTPResponse{
		Key:   resp.Key,
		Value: resp.Value,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(httpResp)
}

// sendError sends an error response to the client
func (s *Server) sendError(w http.ResponseWriter, message string, statusCode int) {
	log.Printf("Error response: %s (status: %d)", message, statusCode)

	httpResp := HTTPResponse{
		Error: message,
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(httpResp)
}

// RegisterPendingRequest tracks a request waiting for quorum
func (s *Server) RegisterPendingRequest(lsn int64, req *Request) chan *Response {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	pending := &PendingRequest{
		request:   req,
		acks:      1, // Primary counts as first ack
		respChan:  make(chan *Response, 1),
		startTime: time.Now(),
		committed: false,
	}

	s.pendingReqs[lsn] = pending
	return pending.respChan
}

// UpdatePendingRequestLSN moves a pending request from temporary LSN to actual LSN
func (s *Server) UpdatePendingRequestLSN(tempLSN, actualLSN int64, req *Request) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	// Move from temp LSN to actual LSN
	if pending, exists := s.pendingReqs[tempLSN]; exists {
		delete(s.pendingReqs, tempLSN)
		pending.request = req
		s.pendingReqs[actualLSN] = pending
	} else {
		// If not found, create new entry
		pending := &PendingRequest{
			request:   req,
			acks:      1, // Primary counts as first ack
			respChan:  make(chan *Response, 1),
			startTime: time.Now(),
			committed: false,
		}
		s.pendingReqs[actualLSN] = pending
	}
}

// RecordAck increments the ack count for a pending request
func (s *Server) RecordAck(lsn int64) (int, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	pending, exists := s.pendingReqs[lsn]
	if !exists {
		return 0, false
	}

	pending.acks++
	return pending.acks, true
}

// CompletePendingRequest marks a request as complete and sends response
func (s *Server) CompletePendingRequest(lsn int64, resp *Response) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	pending, exists := s.pendingReqs[lsn]
	if !exists {
		return
	}

	if !pending.committed {
		pending.committed = true
		pending.respChan <- resp
		close(pending.respChan)
	}

	s.committedReqs = append(s.committedReqs, lsn)
	delete(s.pendingReqs, lsn)
}

// GetPendingRequest retrieves a pending request by LSN
func (s *Server) GetPendingRequest(lsn int64) (*PendingRequest, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	pending, exists := s.pendingReqs[lsn]
	return pending, exists
}

// Checks to see if lsn is committed
func (s *Server) isCommitted(lsn int64) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for _, committedLSN := range s.committedReqs {
		if committedLSN == lsn {
			return true
		}
	}
	return false
}
