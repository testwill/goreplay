package main

import (
	"encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/buger/gor/proto"
)

// KafkaOutput should make producer client.
type KafkaOutput struct {
	config   *KafkaConfig
	producer sarama.AsyncProducer
}

// KafkaOutputFrequency in milliseconds
const KafkaOutputFrequency = 500

// NewKafkaOutput creates instance of kafka producer client.
func NewKafkaOutput(address string, config *KafkaConfig) io.Writer {
	c := sarama.NewConfig()
	c.Producer.RequiredAcks = sarama.WaitForLocal
	c.Producer.Compression = sarama.CompressionSnappy
	c.Producer.Flush.Frequency = KafkaOutputFrequency * time.Millisecond

	brokerList := strings.Split(config.host, ",")

	producer, err := sarama.NewAsyncProducer(brokerList, c)
	if err != nil {
		log.Fatalln("Failed to start Sarama(Kafka) producer:", err)
	}

	o := &KafkaOutput{
		config:   config,
		producer: producer,
	}

	if Settings.verbose {
		// Start infinite loop for tracking errors for kafka producer.
		go o.ErrorHandler()
	}

	return o
}

// ErrorHandler should receive errors
func (o *KafkaOutput) ErrorHandler() {
	for err := range o.producer.Errors() {
		log.Println("Failed to write access log entry:", err)
	}
}

func (o *KafkaOutput) Write(data []byte) (n int, err error) {
	headers := make(map[string]string)
	proto.ParseHeaders([][]byte{data}, func(header []byte, value []byte) bool {
		headers[string(header)] = string(value)
		return true
	})

	req := payloadBody(data)

	kafkaMessage := KafkaMessage{
		ReqURL:     string(proto.Path(req)),
		ReqMethod:  string(proto.Method(req)),
		ReqBody:    string(proto.Body(req)),
		ReqHeaders: headers,
	}
	jsonMessage, _ := json.Marshal(&kafkaMessage)
	message := sarama.StringEncoder(jsonMessage)

	o.producer.Input() <- &sarama.ProducerMessage{
		Topic: o.config.topic,
		Value: message,
	}

	return len(message), nil
}
