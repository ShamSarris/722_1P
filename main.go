package main

// Will contain main script to run set up in command line etc.

import (
	"fmt"
	"log"
	"net/http"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()
	remoting := remote.NewRemote(system, remote.Configure("127.0.0.1", 8081))

	// For now: make one primary node
	isPrimary := true
	rootActor = new Actor
	props := actor.PropsFromProducer(func() actor.Actor { return rootActor })
	pid := system.Root.Spawn(props)

	fmt.Println("Actor started at PID:", pid)
}

