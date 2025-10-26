package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// Helper function to generate random string of given size
func randomString(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Result struct to hold operation results
type Result struct {
	OperationType string
	StartTime     time.Time
	EndTime       time.Time
	Latency       int64
	Value         string
}

// KeyTracker to track written keys in a thread-safe manner
type KeyTracker struct {
	mu   sync.RWMutex
	keys []int
}

func (kt *KeyTracker) Add(key int) {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	kt.keys = append(kt.keys, key)
}

func (kt *KeyTracker) GetRandom() (int, bool) {
	kt.mu.RLock()
	defer kt.mu.RUnlock()
	if len(kt.keys) == 0 {
		return 0, false
	}
	return kt.keys[rand.Intn(len(kt.keys))], true
}

func main() {
	primaryAddr := flag.String("primary", "127.0.0.1:8081", "Address of primary node server to connect to")
	backupAddr_1 := flag.String("backup1", "127.0.0.1:8083", "Address of first backup node server to connect to")
	backupAddr_2 := flag.String("backup2", "127.0.0.1:8085", "Address of second backup node server to connect to")
	rwRatio := flag.Float64("rwratio", 0.5, "Ratio of read to write operations (0.0 to 1.0)")
	writeSize := flag.Int("writesize", 10, "Size of each write operation in bytes")
	readFrom := flag.Int("readfromlog", 0, "0 = Read only from primary, 1 = Read only from backups, 2 = Read from both at random")
	duration := flag.Int("duration", 60, "Duration of the benchmark test in seconds")
	clients := flag.Int("clients", 3, "Number of concurrent clients to simulate")

	flag.Parse()

	// Benchmark logic
	startBenchmark := time.Now()
	endTime := time.Now().Add(time.Duration(*duration) * time.Second)
	resultChan := make(chan Result, *clients*100) // Buffered channel
	var wg sync.WaitGroup
	keyTracker := &KeyTracker{keys: make([]int, 0)}

	// Collect all results in memory
	var results []Result
	var resultsMu sync.Mutex

	// Start result collector
	go func() {
		for result := range resultChan {
			resultsMu.Lock()
			results = append(results, result)
			resultsMu.Unlock()
		}
	}()

	// Start concurrent clients
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			for time.Now().Before(endTime) {
				isWrite := rand.Float64() < *rwRatio
				var key int
				var url, operation string
				var value string

				if isWrite {
					// WRITE: can use any key
					key = rand.Intn(1000000)
					value = randomString(*writeSize)
					url, operation = getURL(true, readFrom, primaryAddr, backupAddr_1, backupAddr_2, key, &value)
				} else {
					// READ: only from written keys
					var ok bool
					key, ok = keyTracker.GetRandom()
					if !ok {
						// No keys written yet, skip this iteration
						continue
					}
					url, operation = getURL(false, readFrom, primaryAddr, backupAddr_1, backupAddr_2, key, nil)
				}

				// Record start time
				startTime := time.Now()

				// Make HTTP request
				var resp *http.Response
				var err error
				if isWrite {
					resp, err = http.Post(url, "text/plain", nil)
				} else {
					resp, err = http.Get(url)
				}

				endTime := time.Now()

				if err != nil {
					log.Printf("Client %d: Error making request: %v", clientID, err)
					continue
				}

				// Read response value
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					value = string(body)
				}
				resp.Body.Close()

				// Track successful writes
				if isWrite {
					keyTracker.Add(key)
				}

				// Send result to channel
				resultChan <- Result{
					OperationType: operation,
					StartTime:     startTime,
					EndTime:       endTime,
					Latency:       endTime.Sub(startTime).Milliseconds(),
					Value:         value,
				}
			}
			log.Printf("Client %d: Benchmark duration ended, stopping new requests\n", clientID)
		}(i)
	}

	// Wait for all clients to finish
	log.Println("Waiting for all clients to finish...")
	wg.Wait()

	// Add grace period to allow in-flight requests to complete
	log.Println("Grace period: waiting 5 seconds for in-flight requests to complete...")
	time.Sleep(5 * time.Second)

	close(resultChan)

	// Wait a moment for result collector to finish
	time.Sleep(100 * time.Millisecond)

	// Calculate throughput
	totalDuration := time.Since(startBenchmark).Seconds()
	throughput := float64(len(results)) / totalDuration

	fmt.Printf("Benchmark completed:\n")
	fmt.Printf("Total operations: %d\n", len(results))
	fmt.Printf("Duration: %.2f seconds\n", totalDuration)
	fmt.Printf("Throughput: %.2f ops/sec\n", throughput)

	// Create CSV file for logging
	file, err := os.Create("latency_log.csv")
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	writer.Write([]string{"Operation Type", "Start Time", "End Time", "Latency (ms)", "Value"})

	// Write all results to CSV
	for _, result := range results {
		writer.Write([]string{
			result.OperationType,
			result.StartTime.Format(time.RFC3339Nano),
			result.EndTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%d", result.Latency),
			result.Value,
		})
	}
}

func getURL(isWrite bool, readFrom *int, primaryAddr, backupAddr_1, backupAddr_2 *string, key int, value *string) (string, string) {
	var operation string
	var url string
	if isWrite {
		// WRITE operation
		operation = "WRITE"
		log.Printf("Sending write with key: %d", key)
		url = fmt.Sprintf("http://%s/%d/%s", *primaryAddr, key, *value)
	} else {
		// READ operation
		operation = "READ"
		switch *readFrom {
		case 0:
			log.Printf("Sending read to primary with key: %d", key)
			url = fmt.Sprintf("http://%s/%d", *primaryAddr, key)
		case 1:
			backup := rand.Intn(2)
			switch backup {
			case 0:
				log.Printf("Sending read to backup 2 with key: %d", key)
				url = fmt.Sprintf("http://%s/%d", *backupAddr_2, key)
			default:
				log.Printf("Sending read to backup 1 with key: %d", key)
				url = fmt.Sprintf("http://%s/%d", *backupAddr_1, key)
			}
		default:
			backup := rand.Intn(3)
			switch backup {
			case 0:
				log.Printf("Sending read to primary with key: %d", key)
				url = fmt.Sprintf("http://%s/%d", *primaryAddr, key)
			case 1:
				log.Printf("Sending read to backup 1 with key: %d", key)
				url = fmt.Sprintf("http://%s/%d", *backupAddr_1, key)
			default:
				log.Printf("Sending read to backup 2 with key: %d", key)
				url = fmt.Sprintf("http://%s/%d", *backupAddr_2, key)
			}
		}
	}
	return url, operation
}
