package main

// Will contain main script to run set up in command line etc.

import (
	//"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

// Generated with Claude Sonnet 3.5. Prompted with "Generate a function to get the local IP address and port for this VM".
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal("Error getting local IP:", err)
	}

	for _, addr := range addrs {
		// Check if it's an IP network
		if ipnet, ok := addr.(*net.IPNet); ok {
			// Skip loopback addresses
			if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	// Fallback to localhost if no other IP found
	return "127.0.0.1"
}

func main() {
	// host := flag.String("host", "127.0.0.1", "Host address to bind")
	port := flag.Int("port", 8080, "Port for actor system")
	httpPort := flag.Int("http", 8081, "Port for HTTP server")
	isPrimary := flag.Bool("primary", false, "Run as primary node")

	flag.Parse()

	// selfIP := getLocalIP()
	selfIP := "127.0.0.1"
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(selfIP, *port)
	remoter := remote.NewRemote(system, remoteConfig)

	switch *isPrimary {
	case true:
		log.Println("Starting as Primary")
		props := actor.PropsFromProducer(func() actor.Actor {
			return &Actor{
				targets:     []*actor.PID{},
				targetNames: make(map[string]string),
				system:      system,
				remoter:     remoter,
				subscribers: 2, // Expecting 2 backups
				isPrimary:   *isPrimary,
				// Log:         make(map[int64]*Request),
				store:    make(map[string]string),
				httpPort: *httpPort,
			}
		})
		remoter.Register("primary", props)
		remoter.Start()

		time.Sleep(1 * time.Second)

		pid, err := system.Root.SpawnNamed(props, "primary")
		if err != nil {
			log.Fatalf("Failed to spawn primary actor: %v", err)
		}
		log.Printf("Backups subscribe to: %s", pid.Address)

		select {}
	case false:
		log.Println("Starting as Backup")
		var primaryIP string
		fmt.Print("Enter the IP address of the primary node: ")
		fmt.Scanln(&primaryIP)

		props := actor.PropsFromProducer(func() actor.Actor {
			return &Actor{
				targets:   []*actor.PID{actor.NewPID(primaryIP, "primary")},
				isPrimary: *isPrimary,
				Log:       make(map[int64]*Request),
				store:     make(map[string]string),
				httpPort:  *httpPort,
			}
		})
		remoter.Register("backup", props)
		remoter.Start()

		pid, err := system.Root.SpawnNamed(props, "backup")
		if err != nil {
			log.Fatalf("Failed to spawn backup actor: %v", err)
		}

		log.Printf("Spawned Backup actor with PID: %v", pid)
		select {}
	default:
		fmt.Println("Unknown role. Use --primary=true or --primary=false")
		os.Exit(1)
	}

	// //Terminate when signal is received
	// sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// <-sigChan
	// fmt.Println()

	// log.Println("Shutting down...")
	// system.Root.Stop(pid)
	// remoter.Shutdown(true)
}
