package kafka_sender

import (
	"flag"
	"fmt"
	"github.com/slinkydeveloper/eventing-thpt/pkg/common"
	"log"
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

var (
	brokerUrl   string
	topicName   string
	msgSize     int
	paceFlag    string
	verbose     bool
	metricsPort int
	fatalf      = log.Fatalf
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&brokerUrl, "bootstrap-server", "", "The kafka bootstrap server url for the event destination.")
	flag.StringVar(&topicName, "topic", "", "The topic for the event destination.")
	flag.IntVar(&metricsPort, "metrics-port", 2112, "Metrics port")
	flag.IntVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")
}

func main() {
	paceSpecs, err := common.ParsePaceSpec(paceFlag)
	if err != nil {
		panic(err)
	}

	var maximumNumberOfMessagesInsideAChannel int
	for _, p := range paceSpecs {
		totalMessages := p.Rps * int(p.Duration.Seconds())
		if totalMessages > maximumNumberOfMessagesInsideAChannel {
			maximumNumberOfMessagesInsideAChannel = totalMessages
		}
	}

	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Version = sarama.V2_0_0_0
	config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	config.ChannelBufferSize = maximumNumberOfMessagesInsideAChannel

	brokers := strings.Split(brokerUrl, ",")

	runProducer(topicName, msgSize, config, brokers, paceSpecs)
}

func runProducer(topic string, messageSize int, config *sarama.Config, brokers []string, paceSpecs []common.PaceSpec) {
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		panic(fmt.Errorf("Failed to create producer: %s", err))
	}
	defer func() {
		_ = producer.Close()
	}()

	defer func() {
		for e := range producer.Errors() {
			println(e)
		}
	}()

	for _, p := range paceSpecs {
		sendBurst(producer.Input(), topic, messageSize, p.Rps, p.Duration)
	}
}

func sendBurst(input chan<- *sarama.ProducerMessage, topic string, messageSize int, throughput int, duration time.Duration) {
	thptTickerInterval := time.Duration((1 / float64(throughput)) * 1000 * 1000 * 1000)
	thptTicker := time.NewTicker(thptTickerInterval)
	endTicker := time.NewTicker(duration)
	for {
		select {
		case <-thptTicker.C:
			input <- generateMessage(topicName, messageSize)
		case <-endTicker.C:
			return
		}
	}
}

func generateMessage(topic string, messageSize int) *sarama.ProducerMessage {
	payload := common.GenerateRandByteArray(messageSize)
	return &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}
}
