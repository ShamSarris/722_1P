package main

// Will contain actor definitions and message/reply logic
import (
	"sync"

	"distributed/messages"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type Actor struct { // TODO: Break into primaryActor and backupActor
	targets     []*actor.PID
	targetNames map[string]string
	system      *actor.ActorSystem
	subscribers int
	remoter     *remote.Remote
	mu          sync.Mutex
	isPrimary   bool
	log         map[int64]string  // LSN => Request  (key, value)
	store       map[string]string // Request (key => value)
}

func (a *Actor) Receive(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *actor.Started: // TODO: Maybe here add StartServer() if primary: a.startServer()
		if !a.isPrimary {
			// If backup, Subscribe to the primary actor
			for _, target := range a.targets {
				ctx.Request(target, &messages.Subscribe{})
			}
		}
	}
}

func role(isPrimary bool) string {
	if isPrimary {
		return "Primary"
	}
	return "Backup"
}
