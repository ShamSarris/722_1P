package main

// Will contain actor definitions and message/reply logic
import (
	"fmt"
	"log"
	"sync"

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
	mu          sync.Mutex
	isPrimary   bool
	httpPort    int
	Log         map[int64]string  // LSN => Request  (key, value) (Note capital L)
	store       map[string]string // Requests (key => value)
}

func (a *Actor) Receive(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *actor.Started:
		if !a.isPrimary {
			// If backup, Subscribe to the primary actor
			for _, target := range a.targets {
				ctx.Request(target, &messages.Subscribe{})
			}
		}

		// On all machines, start server
		Server := NewServer(a, a.httpPort)
		Server.Start()
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
	}

}

func role(isPrimary bool) string {
	if isPrimary {
		return "Primary"
	}
	return "Backup"
}
