package main

import (
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"

	"distributed/messages"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type Actor struct {
	targets        []*actor.PID
	targetNames    map[string]string
	system         *actor.ActorSystem
	subscribers    int
	remoter        *remote.Remote
	Mu             sync.Mutex // Guards store + log
	isPrimary      bool
	httpPort       int
	Server         *Server
	Log            map[int64]*Request // LSN => Request  (key, value) (Note capital L)
	store          map[string]string  // Requests (key => value)
	lsn            atomic.Int64       // Monotonically increasing log sequence number
	serverStarted  bool
	ctx            actor.Context      // Store context for use in write method
	lastAppliedLSN atomic.Int64       // Track the last LSN applied to store
	pendingCommits map[int64]*Request // Queue of commits waiting for previous LSN
	pendingMu      sync.Mutex         // Guards pendingCommits
	// firstRun       bool               // To track first run for testing
}

func (a *Actor) Receive(ctx actor.Context) {
	a.ctx = ctx // Store context for later use
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		// Initialize pendingCommits if not already done
		if a.pendingCommits == nil {
			a.pendingCommits = make(map[int64]*Request)
		}

		if !a.isPrimary {
			// If backup, Subscribe to the primary actor
			for _, target := range a.targets {
				ctx.Request(target, &messages.Subscribe{})
			}
		}

		// On all machines, start server only once
		if a.Server == nil {
			a.Server = NewServer(a, a.httpPort)
		}
		if !a.serverStarted {
			a.Server.Start()
			a.serverStarted = true
		}
	case *messages.Subscribe:
		senderPID := ctx.Sender()
		log.Printf("%s: Received Subscribe message from %s\n", role(a.isPrimary), senderPID.String())
		a.targets = append(a.targets, senderPID)
		name := fmt.Sprintf("Backup%d", len(a.targets))
		a.targetNames[senderPID.String()] = name
		log.Printf("%s: Current targets: %v\n", role(a.isPrimary), a.targets)
		if len(a.targets) >= a.subscribers { // Expected backups compared to actual
			log.Printf("%s: All backups have subscribed. Ready to process requests.\n", role(a.isPrimary))
		}
	case *messages.Write:
		// Step 4) Backup receives write request from primary
		senderStr := "<unknown>"
		if ctx.Sender() != nil {
			senderStr = ctx.Sender().String()
		}
		log.Printf("%s: Received Write(LSN=%d, Key=%s, Value=%s) from %s\n",
			role(a.isPrimary), msg.Lsn, msg.Key, msg.Val, senderStr)
		a.Mu.Lock()                // Guarding Log
		a.Log[msg.Lsn] = &Request{ // Remember requested LSN in Log
			Type: "WRITE",
			Key:  msg.Key,
			Val:  msg.Val,
			LSN:  msg.Lsn,
		}
		a.Mu.Unlock()
		// Only send Ack if we have a valid sender
		if ctx.Sender() != nil {
			ctx.Request(ctx.Sender(), &messages.Ack{Lsn: msg.Lsn}) // Tell primary we logged the requested LSN
		}
	case *messages.Ack:
		if a.isPrimary {
			// Step 5) Primary receives Ack from backup
			log.Printf("Primary: Received Ack(LSN=%d) from %s\n", msg.Lsn, ctx.Sender().String())
			acks, exists := a.Server.RecordAck(msg.Lsn)

			if exists && acks >= int(math.Ceil(float64(a.subscribers+1)/2.0)) { // Quorum reached; Quorum is subscribers + primary / 2 rounded up
				// Check if we can apply this LSN (previous must be applied)
				lastApplied := a.lastAppliedLSN.Load()

				if msg.Lsn == lastApplied+1 {
					// We can apply this LSN immediately
					a.applyLSNToPrimary(msg.Lsn)

					// Now check if any pending LSNs can be applied
					a.applyPendingLSNs()
				} else if msg.Lsn > lastApplied+1 {
					// Queue this for later - previous LSN not applied yet
					log.Printf("Primary: Queuing LSN %d (waiting for LSN %d to be applied first)\n", msg.Lsn, lastApplied+1)
					toCom, exist := a.Server.GetPendingRequest(msg.Lsn)
					if exist {
						a.pendingMu.Lock()
						a.pendingCommits[msg.Lsn] = toCom.request
						a.pendingMu.Unlock()
					}
				} else {
					// LSN already applied, ignore
					log.Printf("Primary: LSN %d already applied (lastApplied=%d)\n", msg.Lsn, lastApplied)
				}
			}
		}
	case *messages.Commit:
		log.Printf("%s: Received Commit(LSN=%d) from %s\n", role(a.isPrimary), msg.Lsn, ctx.Sender().String())

		// Check if we can apply this LSN
		lastApplied := a.lastAppliedLSN.Load()

		if msg.Lsn == lastApplied+1 {
			// Apply immediately
			a.applyLSNToBackup(msg.Lsn)

			// Check if any queued commits can now be applied
			a.applyPendingCommitsToBackup()
		} else if msg.Lsn > lastApplied+1 {
			// Queue for later
			log.Printf("%s: Queuing Commit for LSN %d (waiting for LSN %d)\n",
				role(a.isPrimary), msg.Lsn, lastApplied+1)
			a.Mu.Lock()
			if req, exists := a.Log[msg.Lsn]; exists {
				a.pendingMu.Lock()
				a.pendingCommits[msg.Lsn] = req
				a.pendingMu.Unlock()
			}
			a.Mu.Unlock()
		} else {
			// Already applied
			log.Printf("%s: LSN %d already applied (lastApplied=%d)\n",
				role(a.isPrimary), msg.Lsn, lastApplied)
		}

	case *messages.Read:
		log.Printf("%s: Received Read(LSN=%d, Key=%s) from %s\n", role(a.isPrimary), msg.Lsn, msg.Request, ctx.Sender().String())
		a.Mu.Lock()
		a.Log[msg.Lsn] = &Request{
			Type: "READ",
			Key:  msg.Request,
			LSN:  msg.Lsn,
		}
		a.Mu.Unlock()

		// Update lastAppliedLSN for READ operations on backups
		// READs still consume LSN slots and must be tracked for ordering
		if !a.isPrimary {
			a.lastAppliedLSN.Store(msg.Lsn)
			log.Printf("%s: Updated lastAppliedLSN to %d (READ operation)\n", role(a.isPrimary), msg.Lsn)
		}

		ctx.Send(ctx.Sender(), &messages.Ack{Lsn: msg.Lsn}) // Ack but expect no commit msg back
	}
}

// applyLSNToPrimary applies a single LSN to the primary's store
func (a *Actor) applyLSNToPrimary(lsn int64) {
	toCom, exist := a.Server.GetPendingRequest(lsn)
	if !exist {
		log.Printf("Primary: Cannot apply LSN %d - not found in pending requests\n", lsn)
		return
	}

	a.Mu.Lock()
	defer a.Mu.Unlock()

	if toCom.request.Type == "WRITE" {
		// Send commit to backups
		for _, target := range a.targets {
			log.Printf("Primary: Sending Commit(LSN=%d) to %s\n", lsn, target.String())
			a.ctx.Send(target, &messages.Commit{Lsn: lsn})
		}

		// Apply to store
		a.store[toCom.request.Key] = toCom.request.Val
		log.Printf("Primary: Applied LSN %d (Key=%s, Value=%s) to store\n", lsn, toCom.request.Key, toCom.request.Val)

		// Update last applied
		a.lastAppliedLSN.Store(lsn)

		// Send response to client
		a.Server.CompletePendingRequest(lsn, &Response{
			Success: true,
			Key:     toCom.request.Key,
			Value:   toCom.request.Val,
			Error:   "",
		})
	} else {
		// Read operation
		val, exists := a.store[toCom.request.Key]

		// Update last applied
		a.lastAppliedLSN.Store(lsn)

		if exists {
			a.Server.CompletePendingRequest(lsn, &Response{
				Success: true,
				Key:     toCom.request.Key,
				Value:   val,
				Error:   "",
			})
		} else {
			a.Server.CompletePendingRequest(lsn, &Response{
				Success: false,
				Key:     toCom.request.Key,
				Value:   "",
				Error:   "Key not found",
			})
		}
	}
}

// applyPendingLSNs checks and applies any queued LSNs that can now be applied
func (a *Actor) applyPendingLSNs() {
	for {
		lastApplied := a.lastAppliedLSN.Load()
		nextLSN := lastApplied + 1

		a.pendingMu.Lock()
		_, hasPending := a.pendingCommits[nextLSN]
		a.pendingMu.Unlock()

		if !hasPending {
			// No more consecutive LSNs to apply
			break
		}

		log.Printf("Primary: Applying queued LSN %d\n", nextLSN)
		a.applyLSNToPrimary(nextLSN)

		// Remove from pending
		a.pendingMu.Lock()
		delete(a.pendingCommits, nextLSN)
		a.pendingMu.Unlock()
	}
}

// applyLSNToBackup applies a single LSN to the backup's store
func (a *Actor) applyLSNToBackup(lsn int64) {
	a.Mu.Lock()
	req, exists := a.Log[lsn]
	a.Mu.Unlock()

	if !exists {
		log.Printf("%s: Cannot apply LSN %d - not found in log\n", role(a.isPrimary), lsn)
		return
	}

	a.Mu.Lock()
	a.store[req.Key] = req.Val
	a.Mu.Unlock()

	// Update last applied
	a.lastAppliedLSN.Store(lsn)

	log.Printf("%s: Applied LSN %d (Key=%s, Value=%s) to store\n", role(a.isPrimary), lsn, req.Key, req.Val)
}

// applyPendingCommitsToBackup checks and applies any queued commits
func (a *Actor) applyPendingCommitsToBackup() {
	for {
		lastApplied := a.lastAppliedLSN.Load()
		nextLSN := lastApplied + 1

		a.pendingMu.Lock()
		_, hasPending := a.pendingCommits[nextLSN]
		a.pendingMu.Unlock()

		if !hasPending {
			// No more consecutive LSNs to apply
			break
		}

		log.Printf("%s: Applying queued commit for LSN %d\n", role(a.isPrimary), nextLSN)
		a.applyLSNToBackup(nextLSN)

		// Remove from pending
		a.pendingMu.Lock()
		delete(a.pendingCommits, nextLSN)
		a.pendingMu.Unlock()
	}
}

func (a *Actor) write(req *Request) {
	// Step 1) Atomically increment and get new LSN
	// if a.firstRun {
	// 	a.firstRun = false
	// 	req.LSN = 2
	// } else {
	// 	req.LSN = 1
	// }
	req.LSN = a.lsn.Add(1) // Increment and get new LSN

	// Step 2) Register pending request with correct LSN
	a.Server.UpdatePendingRequestLSN(-1, req.LSN, req)

	// Step 3) Send initial Accept (Write) message to all backups
	accept := &messages.Write{
		Lsn: req.LSN,
		Key: req.Key,
		Val: req.Val,
	}

	// Log the request in the primary's log
	a.Mu.Lock()
	a.Log[req.LSN] = req
	a.Mu.Unlock()

	for _, target := range a.targets {
		log.Printf("%s: Sending Write(LSN=%d, Key=%s, Value=%s) to %s\n",
			role(a.isPrimary), accept.Lsn, accept.Key, accept.Val, target.String())
		a.ctx.Request(target, accept)
	}
}

func (a *Actor) read(req *Request) {
	if !a.isPrimary {
		a.Mu.Lock()
		defer a.Mu.Unlock()

		if val, exists := a.store[req.Key]; exists {
			// Return the value if key exists
			log.Printf("Backup: Read Key=%s, Value=%s from store\n", req.Key, val)
			a.Server.CompletePendingRequest(req.LSN, &Response{
				Success: true,
				Key:     req.Key,
				Value:   val,
				Error:   "",
			})
		} else {
			// Key not found
			log.Printf("Backup: Read Key=%s - not found in store\n", req.Key)
			a.Server.CompletePendingRequest(req.LSN, &Response{
				Success: false,
				Key:     req.Key,
				Value:   "",
				Error:   "Key not found",
			})
		}
	} else {
		req.LSN = a.lsn.Add(1)
		a.Server.UpdatePendingRequestLSN(-1, req.LSN, req)

		// Log the request in the primary's log
		a.Mu.Lock()
		a.Log[req.LSN] = req
		a.Mu.Unlock()

		accept := &messages.Read{
			Lsn:     req.LSN,
			Request: req.Key,
		}

		for _, target := range a.targets {
			log.Printf("Primary: Sending Read(LSN=%d, Key=%s) to %s\n",
				accept.Lsn, accept.Request, target.String())
			a.ctx.Request(target, accept)
		}
	}
}

func role(isPrimary bool) string {
	if isPrimary {
		return "Primary"
	}
	return "Backup"
}
