package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"vortex-cache/internal/resp"
)

type LatencyResult struct {
	OpsPerSec float64
	Min       time.Duration
	P50       time.Duration
	P95       time.Duration
	P99       time.Duration
	P999      time.Duration
	Max       time.Duration
	Avg       time.Duration
}

func calculateMetrics(durations []time.Duration, totalTime time.Duration) LatencyResult {
	if len(durations) == 0 {
		return LatencyResult{}
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var sum int64
	for _, d := range durations {
		sum += d.Nanoseconds()
	}

	n := len(durations)
	avg := time.Duration(sum / int64(n))

	p50 := durations[int(float64(n)*0.50)]
	p95 := durations[int(float64(n)*0.95)]
	p99 := durations[int(float64(n)*0.99)]
	p999Index := int(float64(n) * 0.999)
	if p999Index >= n {
		p999Index = n - 1
	}
	p999 := durations[p999Index]

	opsPerSec := float64(n) / totalTime.Seconds()

	return LatencyResult{
		OpsPerSec: opsPerSec,
		Min:       durations[0],
		P50:       p50,
		P95:       p95,
		P99:       p99,
		P999:      p999,
		Max:       durations[n-1],
		Avg:       avg,
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:6379", "Target Vortex TCP server address")
	concurrency := flag.Int("c", 50, "Number of concurrent clients")
	requests := flag.Int("n", 100000, "Total number of requests")
	payloadSize := flag.Int("size", 64, "Payload size in bytes")
	hotkey := flag.Bool("hotkey", false, "Simulate hot-key contention (all workers access same key)")
	readRatio := flag.Float64("ratio", 0.9, "Read ratio (e.g. 0.9 = 90% GET, 10% SET)")
	csvFormat := flag.Bool("csv", false, "Output results in CSV format")
	flag.Parse()

	payload := make([]byte, *payloadSize)
	for i := range payload {
		payload[i] = byte('A' + (i % 26))
	}

	requestsPerWorker := *requests / *concurrency
	actualTotalRequests := requestsPerWorker * *concurrency

	var wg sync.WaitGroup
	wg.Add(*concurrency)

	latencies := make([][]time.Duration, *concurrency)
	var completedRequests atomic.Int64

	startTime := time.Now()

	for workerID := 0; workerID < *concurrency; workerID++ {
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", *addr)
			if err != nil {
				fmt.Printf("Worker %d failed to connect: %v\n", id, err)
				return
			}
			defer conn.Close()

			reader := resp.NewReader(conn)
			writer := resp.NewWriter(conn)

			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			workerLatencies := make([]time.Duration, 0, requestsPerWorker)

			for i := 0; i < requestsPerWorker; i++ {
				var key string
				if *hotkey {
					key = "hotkey:contention:1"
				} else {
					key = fmt.Sprintf("key:spread:%d:%d", id, i%1000)
				}

				isRead := r.Float64() < *readRatio

				opStart := time.Now()

				if isRead {
					_ = writer.WriteArray([]resp.Value{
						resp.NewBulkStringFromString("GET"),
						resp.NewBulkStringFromString(key),
					})
				} else {
					_ = writer.WriteArray([]resp.Value{
						resp.NewBulkStringFromString("SET"),
						resp.NewBulkStringFromString(key),
						resp.NewBulkString(payload),
					})
				}
				_ = writer.Flush()

				_, err := reader.ReadValue()
				if err != nil {
					fmt.Printf("Worker %d read error: %v\n", id, err)
					return
				}

				opDur := time.Since(opStart)
				workerLatencies = append(workerLatencies, opDur)
				completedRequests.Add(1)
			}

			latencies[id] = workerLatencies
		}(workerID)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	allLatencies := make([]time.Duration, 0, actualTotalRequests)
	for _, l := range latencies {
		allLatencies = append(allLatencies, l...)
	}

	metrics := calculateMetrics(allLatencies, totalDuration)

	modeStr := "Spread Key"
	if *hotkey {
		modeStr = "Hot Key Contention"
	}

	if *csvFormat {
		fmt.Printf("Timestamp,Mode,Concurrency,Requests,PayloadBytes,OpsPerSec,MinMs,P50Ms,P95Ms,P99Ms,P999Ms,MaxMs,AvgMs\n")
		fmt.Printf("%s,%s,%d,%d,%d,%.2f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f\n",
			time.Now().Format(time.RFC3339),
			modeStr,
			*concurrency,
			len(allLatencies),
			*payloadSize,
			metrics.OpsPerSec,
			float64(metrics.Min.Microseconds())/1000.0,
			float64(metrics.P50.Microseconds())/1000.0,
			float64(metrics.P95.Microseconds())/1000.0,
			float64(metrics.P99.Microseconds())/1000.0,
			float64(metrics.P999.Microseconds())/1000.0,
			float64(metrics.Max.Microseconds())/1000.0,
			float64(metrics.Avg.Microseconds())/1000.0,
		)
		return
	}

	fmt.Println("\n=======================================================")
	fmt.Printf("VORTEX CACHE BENCHMARK REPORT (%s)\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("=======================================================")
	fmt.Printf("Mode:               %s\n", modeStr)
	fmt.Printf("Concurrency:        %d clients\n", *concurrency)
	fmt.Printf("Total Requests:     %d\n", len(allLatencies))
	fmt.Printf("Payload Size:       %d bytes\n", *payloadSize)
	fmt.Printf("Read/Write Ratio:   %.0f%% GET / %.0f%% SET\n", *readRatio*100, (1-*readRatio)*100)
	fmt.Printf("Total Time:         %v\n", totalDuration)
	fmt.Printf("Throughput:         %.2f ops/sec\n", metrics.OpsPerSec)
	fmt.Println("-------------------------------------------------------")
	fmt.Println("LATENCY PERCENTILES:")
	fmt.Printf("  Min:   %8.3f ms (%d µs)\n", float64(metrics.Min.Microseconds())/1000.0, metrics.Min.Microseconds())
	fmt.Printf("  p50:   %8.3f ms (%d µs)\n", float64(metrics.P50.Microseconds())/1000.0, metrics.P50.Microseconds())
	fmt.Printf("  p95:   %8.3f ms (%d µs)\n", float64(metrics.P95.Microseconds())/1000.0, metrics.P95.Microseconds())
	fmt.Printf("  p99:   %8.3f ms (%d µs)\n", float64(metrics.P99.Microseconds())/1000.0, metrics.P99.Microseconds())
	fmt.Printf("  p999:  %8.3f ms (%d µs)\n", float64(metrics.P999.Microseconds())/1000.0, metrics.P999.Microseconds())
	fmt.Printf("  Max:   %8.3f ms (%d µs)\n", float64(metrics.Max.Microseconds())/1000.0, metrics.Max.Microseconds())
	fmt.Printf("  Avg:   %8.3f ms (%d µs)\n", float64(metrics.Avg.Microseconds())/1000.0, metrics.Avg.Microseconds())
	fmt.Println("=======================================================")
}
