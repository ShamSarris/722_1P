package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// TODO: Fix key generation; Writes can create new key or update old; Reads should only read existing keys

// Helper function to generate random string of given size
func randomString(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func main() {
	primaryAddr := flag.String("primary", "127.0.0.1:8081", "Address of primary node server to connect to")
	backupAddr_1 := flag.String("backup1", "127.0.0.1:8083", "Address of first backup node server to connect to")
	backupAddr_2 := flag.String("backup2", "127.0.0.1:8085", "Address of second backup node server to connect to")
	rwRatio := flag.Float64("rwratio", 0.8, "Ratio of read to write operations (0.0 to 1.0)")
	writeSize := flag.Int("writesize", 10, "Size of each write operation in bytes")
	readFrom := flag.Int("readfromlog", 0, "0 = Read only from primary, 1 = Read only from backups, 2 = Read from both at random")
	duration := flag.Int("duration", 60, "Duration of the benchmark test in seconds")
	clients := flag.Int("clients", 1, "Number of concurrent clients to simulate")

	flag.Parse()

	// Create CSV file for logging latencies
	file, err := os.Create("latency_log.csv")
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	writer.Write([]string{"Client", "Operation", "Latency", "isFailed"})

	// Benchmark logic
	endTime := time.Now().Add(time.Duration(*duration) * time.Second)
	for i := 0; i < *clients; i++ {
		for time.Now().Before(endTime) {
			var operation string
			var url string
			if rand.Float64() < *rwRatio {
				// WRITE operation
				operation = "WRITE"
				log.Printf("Sending write with key: %d", i)
				url = fmt.Sprintf("http://%s/%d/%s", *primaryAddr, i, randomString(*writeSize))
			} else {
				// READ operation
				operation = "READ"
				switch *readFrom {
				case 0:
					log.Printf("Sending read to primary with key: %d", i)
					url = fmt.Sprintf("http://%s/%d", *primaryAddr, i)
				case 1:
					backup := rand.Intn(2)
					switch backup {
					case 0:
						log.Printf("Sending read to backup 2 with key: %d", i)
						url = fmt.Sprintf("http://%s/%d", *backupAddr_2, i)
					default:
						log.Printf("Sending read to backup 1 with key: %d", i)
						url = fmt.Sprintf("http://%s/%d", *backupAddr_1, i)
					}
				default:
					backup := rand.Intn(2)
					switch backup {
					case 0:
						log.Printf("Sending read to primary with key: %d", i)
						url = fmt.Sprintf("http://%s/%d", *primaryAddr, i)
					case 1:
						log.Printf("Sending read to backup 1 with key: %d", i)
						url = fmt.Sprintf("http://%s/%d", *backupAddr_1, i)
					default:
						log.Printf("Sending read to backup 2 with key: %d", i)
						url = fmt.Sprintf("http://%s/%d", *backupAddr_2, i)
					}
				}
			}

			startTime := time.Now()
			resp, err := http.Get(url)
			var isFailed = false
			if err != nil {
				fmt.Println("Error making request:", err)
				continue
			}
			resp.Body.Close()
			latency := time.Since(startTime).Milliseconds()

			// Log latency
			writer.Write([]string{fmt.Sprintf("%d", i), operation, fmt.Sprintf("%d", latency), fmt.Sprintf("%t", isFailed)})
		}
	}
}
