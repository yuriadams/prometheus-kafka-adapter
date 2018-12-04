package kafka

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/Shopify/sarama"
	"github.com/prometheus/common/model"

	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

// NewClient Mestre Bimba Blablabla
func NewClient(brokers string, verbose bool, certFile string, keyFile string, caFile string, verifySsl bool) *Client {

	if verbose {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}

	if brokers == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	brokerList := strings.Split(brokers, ",")
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	client := &Client{
		DataCollector:     newDataCollector(brokerList, certFile, keyFile, caFile, verifySsl),
		AccessLogProducer: newAccessLogProducer(brokerList, certFile, keyFile, caFile, verifySsl),
	}

	return client
}

func createTlsConfiguration(certFile string, keyFile string, caFile string, verifySsl bool) (t *tls.Config) {
	if certFile != "" && keyFile != "" && caFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: verifySsl,
		}
	}
	// will be nil by default if nothing is provided
	return t
}

type Client struct {
	DataCollector     sarama.SyncProducer
	AccessLogProducer sarama.AsyncProducer
}

type Metric struct {
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"@timestamp"`
	Labels    map[string]interface{} `json:"labels"`
}

func (c *Client) Close() error {
	if err := c.DataCollector.Close(); err != nil {
		log.Println("Failed to shut down data collector cleanly", err)
	}

	if err := c.AccessLogProducer.Close(); err != nil {
		log.Println("Failed to shut down access log producer cleanly", err)
	}

	return nil
}

func (c *Client) Write(samples model.Samples) error {
	c.produce(samples)

	defer func() {
		if err := c.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	return nil
}

func (c *Client) produce(samples model.Samples) error {
	for _, s := range samples {
		v := float64(s.Value)

		if math.IsNaN(v) || math.IsInf(v, 0) {
			fmt.Printf("cannot send value %f to Kafka, skipping sample %#v", v, s)
			continue
		}

		document := Metric{v, s.Timestamp.Time(), buildLabels(s.Metric)}
		documentJSON, err := json.Marshal(document)

		if err != nil {
			fmt.Printf("error while marshaling document, err: %v", err)
			continue
		}

		partition, offset, err := c.DataCollector.SendMessage(&sarama.ProducerMessage{
			Topic: "prometheus",
			Value: sarama.StringEncoder(documentJSON),
		})

		if err != nil {
			return fmt.Errorf("Failed to store your data:, %s", err)
		}

		fmt.Printf("Your data is stored with unique identifier important/%d/%d", partition, offset)
	}

	return nil
}

func buildLabels(m model.Metric) map[string]interface{} {
	fields := make(map[string]interface{}, len(m))
	for l, v := range m {
		fields[string(l)] = string(v)
	}
	return fields
}

func newDataCollector(brokerList []string, certFile string, keyFile string, caFile string, verifySsl bool) sarama.SyncProducer {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	tlsConfig := createTlsConfiguration(certFile, keyFile, caFile, verifySsl)
	if tlsConfig != nil {
		config.Net.TLS.Config = tlsConfig
		config.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	return producer
}

func newAccessLogProducer(brokerList []string, certFile string, keyFile string, caFile string, verifySsl bool) sarama.AsyncProducer {

	// For the access log, we are looking for AP semantics, with high throughput.
	// By creating batches of compressed messages, we reduce network I/O at a cost of more latency.
	config := sarama.NewConfig()
	tlsConfig := createTlsConfiguration(certFile, keyFile, caFile, verifySsl)
	if tlsConfig != nil {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = tlsConfig
	}
	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms

	producer, err := sarama.NewAsyncProducer(brokerList, config)
	if err != nil {
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	// We will just log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	go func() {
		for err := range producer.Errors() {
			log.Println("Failed to write access log entry:", err)
		}
	}()

	return producer
}

// Name identifies the client as an kafka client.
func (c Client) Name() string {
	return "kafka"
}
