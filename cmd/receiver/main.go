/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/slinkydeveloper/eventing-thpt/pkg/common"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

// flags for the image
var (
	verbose         bool
	maxThptExpected int
	metricsPort int
	port int
	fatalf          = log.Fatalf
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.IntVar(&metricsPort, "metrics-port", 2112, "Metrics port")
	flag.IntVar(&port, "port", 8080, "Port")
	flag.IntVar(&maxThptExpected, "max-throughput-expected", 0, "Max throughput expected in rps. This is required to preallocate as much memory as possible")
}

func main() {
	// parse the command line flags
	flag.Parse()

	if verbose {
		printf("Receiver configuration")
		flag.VisitAll(func(i *flag.Flag) {
			printf("%v: %v", i.Name, i.Value)
		})
	}

	// Disable the gc
	debug.SetGCPercent(-1)

	if maxThptExpected <= 0 {
		log.Fatalf("max-throughput-expected must be > 0")
	}

	printf("--- BENCHMARK ---")

	printf("Starting serving metrics")

	receivedCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "received_total",
		Help: "The total number of processed events",
	})

	prometheus.MustRegister(receivedCount)

	// Start metrics serve
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)
	}()

	printf("Starting CloudEvents receiver")

	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithBinaryEncoding(),
		cloudevents.WithPort(port),
	)
	if err != nil {
		fatalf("failed to create transport: %v\n", err)
	}
	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		fatalf("failed to create client: %v\n", err)
	}

	end := make(chan struct{})

	go c.StartReceiver(context.Background(), func(event cloudevents.Event) {
		eventType := event.Type()
		receivedCount.Inc()
		if eventType == common.BenchmarkEndEventType {
			end <- struct{}{}
		} else if eventType == common.BenchmarkGcEventType {
			runtime.GC()
		}
	})

	<- end

	printf("Closing")

	time.Sleep(2 * time.Minute)
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
}
