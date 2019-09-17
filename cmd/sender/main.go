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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/slinkydeveloper/eventing-thpt/pkg/common"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	vegeta "github.com/tsenart/vegeta/lib"
)

// flags for the image
var (
	sinkURL     string
	msgSize     int
	workers     uint64
	paceFlag    string
	verbose     bool
	metricsPort int
	fatalf      = log.Fatalf
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
	flag.IntVar(&metricsPort, "metrics-port", 2112, "Metrics port")
	flag.IntVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")
	flag.Uint64Var(&workers, "workers", 1, "Number of vegeta workers")
}

type attackSpec struct {
	pacer    vegeta.Pacer
	duration time.Duration
}

func (a attackSpec) Attack(i int, targeter vegeta.Targeter, attacker *vegeta.Attacker, c prometheus.Counter, ceClient cloudevents.Client) {
	printf("Starting attack %d° with pace %v rps for %v seconds", i+1, a.pacer, a.duration)
	res := attacker.Attack(targeter, a.pacer, a.duration, fmt.Sprintf("%s-attack-%d", common.EventSource, i))
	for _ = range res {
		c.Inc()
	}

	sendGCEvent(ceClient)
	runtime.GC()
	time.Sleep(1 * time.Second)
}

func main() {
	// parse the command line flags
	flag.Parse()

	if verbose {
		printf("Sender configuration")
		flag.VisitAll(func(i *flag.Flag) {
			printf("%v: %v", i.Name, i.Value)
		})
	}

	// Disable the gc
	debug.SetGCPercent(-1)

	if paceFlag == "" {
		fatalf("pace not set!")
	}

	if os.Getenv("SINK") != "" {
		sinkURL = os.Getenv("SINK")
	}

	if sinkURL == "" {
		fatalf("sink not set!")
	}

	pacerSpecs, err := common.ParsePaceSpec(paceFlag)
	if err != nil {
		fatalf("%+v", err)
	}

	// --- Ce client to send stop/gc events

	t, err := cloudevents.NewHTTPTransport(
		cloudevents.WithTarget(sinkURL),
		cloudevents.WithBinaryEncoding(),
	)
	if err != nil {
		fatalf("failed to create transport: %v\n", err)
	}
	ceClient, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)
	if err != nil {
		fatalf("failed to create client: %v\n", err)
	}

	printf("--- BENCHMARK ---")

	printf("Starting serving metrics")

	sentCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sent_total",
		Help: "The total number of processed events",
	})

	prometheus.MustRegister(sentCount)

	// Start metrics serve
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)
	}()

	// sleep 30 seconds before sending the events
	time.Sleep(30 * time.Second)

	targeter := common.NewCloudEventsTargeter(sinkURL, msgSize, common.BenchmarkContinueEventType, common.EventSource, "binary").VegetaTargeter()

	attacks := make([]attackSpec, len(pacerSpecs))
	var totalBenchmarkDuration time.Duration = 0

	for i, ps := range pacerSpecs {
		attacks[i] = attackSpec{pacer: vegeta.ConstantPacer{Freq: ps.Rps, Per: time.Second}, duration: ps.Duration}
		printf("%d° pace: %d rps for %v seconds", i+1, ps.Rps, ps.Duration)
		totalBenchmarkDuration = totalBenchmarkDuration + ps.Duration
	}

	printf("Total benchmark duration: %v", totalBenchmarkDuration.Seconds())

	printf("Starting benchmark")

	attacker := vegeta.NewAttacker(
		vegeta.Workers(workers),
		vegeta.KeepAlive(true),
	)

	for i, f := range attacks {
		f.Attack(i, targeter, attacker, sentCount, ceClient)
	}

	sendStopEvent(ceClient)

	printf("--- END BENCHMARK ---")

	time.Sleep(2 * time.Minute)
}

func sendGCEvent(ceClient cloudevents.Client) {
	event := cloudevents.NewEvent()
	event.SetID("-1")
	event.SetType(common.BenchmarkGcEventType)
	event.SetSource(common.EventSource)

	_, _ = ceClient.Send(context.TODO(), event)
}

func sendStopEvent(ceClient cloudevents.Client) {
	event := cloudevents.NewEvent()
	event.SetID("-1")
	event.SetType(common.BenchmarkEndEventType)
	event.SetSource(common.EventSource)

	_, _ = ceClient.Send(context.TODO(), event)
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
}
