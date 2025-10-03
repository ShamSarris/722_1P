package main

// Will contain main script to run set up in command line etc.

import (
	//"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host address to bind")
	port := flag.Int("port", 8080, "Port for actor system")
	httpPort := flag.Int("http", 8081, "Port for HTTP server")
	isPrimary := flag.Bool("primary", false, "Run as primary node") 

	flag.Parse()

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(*host, *port)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	actorInstance := &Actor{
		//targets:     targets,
		//targetNames: targetNames,
		system:      system,
		subscribers: 0,
		remoter:     remoter,
		isPrimary:   *isPrimary,
		log:         make(map[int64]string),
		store:       make(map[string]string),
	}

	// For now: make one primary node
	props := actor.PropsFromProducer(func() actor.Actor {
		return actorInstance
	})	
	pid := system.Root.Spawn(props)

	time.Sleep(2 * time.Second)

	httpServer := NewServer(actorInstance, *httpPort)
	httpServer.Start()

	//Terminate when signal is received
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	fmt.Println()

	log.Println("Shutting down...")
	system.Root.Stop(pid)
	remoter.Shutdown(true)}

