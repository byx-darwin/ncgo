// Optional Kafka add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/kafka.go in your project,
// then register the producer / consumer configs and providers with samber/do.
//
// Producer (Writer):
//
//	do.ProvideValue(injector, mwkafka.WriterConfig{
//	    Broker: []string{"kafka-1:9092", "kafka-2:9092"},
//	    Topic:  "events",
//	    AllowAutoTopicCreation: false,
//	})
//	do.Provide(injector, data.NewKafkaWriter)
//
// Consumer (Reader):
//
//	do.ProvideValue(injector, mwkafka.ReaderConfig{
//	    Broker:  []string{"kafka-1:9092", "kafka-2:9092"},
//	    GroupID: "user-service",
//	    Topic:   "events",
//	    MinBytes: 10e3,        // 10 KB
//	    MaxBytes: 10e6,        // 10 MB
//	    MaxWait:  500 * time.Millisecond,
//	})
//	do.Provide(injector, data.NewKafkaReader)
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	mwkafka "github.com/byx-darwin/go-tools/go-middleware/kafka"
)

// KafkaWriter wraps a go-middleware Kafka Writer for producing messages.
type KafkaWriter struct {
	W *mwkafka.Writer
}

// KafkaReader wraps a go-middleware Kafka Consumer for consuming a topic.
type KafkaReader struct {
	R *mwkafka.Consumer
}

// NewKafkaWriter creates a Kafka Writer from mwkafka.WriterConfig via
// go-middleware/kafka and returns a cleanup function for samber/do.
func NewKafkaWriter(cfg mwkafka.WriterConfig) (*KafkaWriter, func(), error) {
	if len(cfg.Broker) == 0 {
		return nil, nil, goerror.
			In("kafka").
			Tags("message-bus", "kafka", "producer", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("mwkafka.WriterConfig.Broker is empty")
	}
	if cfg.Topic == "" {
		return nil, nil, goerror.
			In("kafka").
			Tags("message-bus", "kafka", "producer", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("mwkafka.WriterConfig.Topic is empty")
	}
	w := mwkafka.NewWriter(cfg)
	cleanup := func() { _ = w.Close() }
	return &KafkaWriter{W: w}, cleanup, nil
}

// NewKafkaReader creates a Kafka Consumer from mwkafka.ReaderConfig via
// go-middleware/kafka and returns a cleanup function for samber/do.
func NewKafkaReader(cfg mwkafka.ReaderConfig) (*KafkaReader, func(), error) {
	if len(cfg.Broker) == 0 {
		return nil, nil, goerror.
			In("kafka").
			Tags("message-bus", "kafka", "consumer", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("mwkafka.ReaderConfig.Broker is empty")
	}
	if cfg.Topic == "" {
		return nil, nil, goerror.
			In("kafka").
			Tags("message-bus", "kafka", "consumer", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			With("brokers_count", len(cfg.Broker)).
			New("mwkafka.ReaderConfig.Topic is empty")
	}
	c := mwkafka.NewConsumer(cfg)
	cleanup := func() { _ = c.Close() }
	return &KafkaReader{R: c}, cleanup, nil
}
