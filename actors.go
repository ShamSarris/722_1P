package main

// Will contain actor definitions and message/reply logic
import (
	"fmt"
	"log"
	"sync/atomic"

	"distributed/messages"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type Actor struct {
	targets     []*actor.PID
	targetNames map[string]string
	system      *actor.ActorSystem
	subscribers int
	remoter     *remote.Remote
	// mu          sync.Mutex
	isPrimary     bool
	httpPort      int
	Server        *Server
	Log           map[int64]*Request // LSN => Request  (key, value) (Note capital L)
	store         map[string]string  // Requests (key => value)
	lsn           atomic.Int64       // Monotonically increasing log sequence number
	serverStarted bool
	ctx           actor.Context // Store context for use in write method
}

func (a *Actor) Receive(ctx actor.Context) {
	a.ctx = ctx // Store context for later use
	switch msg := ctx.Message().(type) {
	case *actor.Started:
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
		a.Log[msg.Lsn] = &Request{ // Remember requested LSN in Log
			Key: msg.Key,
			Val: msg.Val,
		}
		// Only send Ack if we have a valid sender
		if ctx.Sender() != nil {
			ctx.Request(ctx.Sender(), &messages.Ack{Lsn: msg.Lsn}) // Tell primary we logged the requested LSN
		}
	case *messages.Ack:
		if a.isPrimary {
			// Step 5) Primary receives Ack from backup
			log.Printf("Primary: Received Ack(LSN=%d) from %s\n", msg.Lsn, ctx.Sender().String())
			acks, exists := a.Server.RecordAck(msg.Lsn)

			if exists && acks >= 2 { // Quorum reached (w/ 2 backups + 1 primary)
				if a.Server.isCommitted(msg.Lsn-1) || msg.Lsn == 1 { // TODO: Not sure if this is right; Need to check previous LSN has been APPLIED to store. If previous hasn't been applied put it in a queue and apply later
					for _, target := range a.targets {
						log.Printf("Primary: Sending Commit(LSN=%d) to %s\n", msg.Lsn, target.String())
						ctx.Send(target, &messages.Commit{Lsn: msg.Lsn}) // Tell backups to commit
					}

					toCom, exist := a.Server.GetPendingRequest(msg.Lsn)
					if exist {
						a.store[toCom.request.Key] = toCom.request.Val // Commit to store
						log.Printf("Primary: Committed LSN %d (Key=%s, Value=%s) to store\n", msg.Lsn, toCom.request.Key, toCom.request.Val)
						a.Server.CompletePendingRequest(msg.Lsn, &Response{
							Success: true,
							Key:     toCom.request.Key,
							Value:   toCom.request.Val,
							Error:   "",
						})
					}
				} else {
					// What to do if previous LSN not committed? Probably add to queue and check queue each write
					log.Printf("!!Previous LSN %d not committed yet. Cannot commit LSN %d\n", msg.Lsn-1, msg.Lsn)
					break
				}
			}
		}
	case *messages.Commit:
		log.Printf("%s: Received Commit(LSN=%d) from %s\n", role(a.isPrimary), msg.Lsn, ctx.Sender().String())
		if req, exists := a.Log[msg.Lsn]; exists { // TODO: also check if the previous LSN has been applied to store
			a.store[req.Key] = req.Val
			log.Printf("%s: Committed LSN %d (Key=%s, Value=%s) to store\n", role(a.isPrimary), msg.Lsn, req.Key, req.Val)
		}
	}
}

func (a *Actor) write(req *Request) {
	// Step 1) Atomically increment and get new LSN
	req.LSN = a.lsn.Add(1) // Increment and get new LSN

	// Step 2) Register pending request with correct LSN
	a.Server.UpdatePendingRequestLSN(-1, req.LSN, req)

	// Step 3) Send initial Accept (Write) message to all backups
	accept := &messages.Write{
		Lsn: req.LSN,
		Key: req.Key,
		Val: req.Val,
	}

	for _, target := range a.targets {
		log.Printf("%s: Sending Write(LSN=%d, Key=%s, Value=%s) to %s\n",
			role(a.isPrimary), accept.Lsn, accept.Key, accept.Val, target.String())
		a.ctx.Request(target, accept)
	}
}

func role(isPrimary bool) string {
	if isPrimary {
		return "Primary"
	}
	return "Backup"
}
