// Optional Kafka add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/kafka.go in your project,
// then register the producer / consumer configs and providers with samber/do.
//
// Producer (Writer):
//
//	do.ProvideValue(injector, &kafka.Writer{
//	    Addr:                   kafka.TCP("kafka-1:9092", "kafka-2:9092"),
//	    Topic:                  "events",
//	    Balancer:               &kafka.Hash{},
//	    MaxAttempts:            10,
//	    WriteBackoffMin:        100 * time.Millisecond,
//	    WriteBackoffMax:        1 * time.Second,
//	    BatchSize:              100,
//	    BatchBytes:             1 << 20, // 1 MiB
//	    BatchTimeout:           10 * time.Millisecond,
//	    ReadTimeout:            10 * time.Second,
//	    WriteTimeout:           10 * time.Second,
//	    RequiredAcks:           kafka.RequireAll,
//	    Async:                  false,
//	    Compression:            kafka.Snappy,
//	    AllowAutoTopicCreation: false,
//	    Transport:              nil, // *kafka.Transport for SASL/TLS
//	    Logger:                 nil,
//	    ErrorLogger:            nil,
//	})
//	do.Provide(injector, data.NewKafkaWriter)
//
// Consumer (Reader):
//
//	do.ProvideValue(injector, kafka.ReaderConfig{
//	    Brokers:                []string{"kafka-1:9092", "kafka-2:9092"},
//	    GroupID:                "user-service",
//	    Topic:                  "events",
//	    Partition:              0,
//	    QueueCapacity:           100,
//	    MinBytes:               10e3, // 10 KB
//	    MaxBytes:               10e6, // 10 MB
//	    MaxWait:                500 * time.Millisecond,
//	    HeartbeatInterval:      3 * time.Second,
//	    CommitInterval:         0, // 0 = sync after each message
//	    PartitionWatchInterval: 5 * time.Second,
//	    WatchPartitionChanges:  true,
//	    SessionTimeout:         30 * time.Second,
//	    RebalanceTimeout:       30 * time.Second,
//	    RetentionTime:          24 * time.Hour,
//	    StartOffset:            kafka.LastOffset,
//	    IsolationLevel:         kafka.ReadCommitted,
//	    MaxAttempts:            3,
//	    Dialer:                 nil, // *kafka.Dialer for SASL/TLS
//	})
//	do.Provide(injector, data.NewKafkaReader)
//
// Full field reference: https://pkg.go.dev/github.com/segmentio/kafka-go
//
// Required dependency:
//
//	go get github.com/segmentio/kafka-go
//	go get github.com/samber/oops

package data

import (
	"github.com/samber/oops"
	"github.com/segmentio/kafka-go"
)

// KafkaWriter wraps a kafka-go Writer for producing messages.
type KafkaWriter struct {
	W *kafka.Writer
}

// KafkaReader wraps a kafka-go Reader for consuming a topic.
type KafkaReader struct {
	R *kafka.Reader
}

// NewKafkaWriter wraps an already-configured *kafka.Writer with a samber/do cleanup.
// The caller passes the fully-populated Writer struct via do.ProvideValue.
func NewKafkaWriter(w *kafka.Writer) (*KafkaWriter, func(), error) {
	if w == nil {
		return nil, nil, oops.
			In("kafka").
			Tags("message-bus", "kafka", "producer", "configuration").
			Code(10308).
			Public("config_invalid").
			New("kafka.Writer is nil")
	}
	if w.Addr == nil {
		return nil, nil, oops.
			In("kafka").
			Tags("message-bus", "kafka", "producer", "configuration").
			Code(10308).
			Public("config_invalid").
			New("kafka.Writer.Addr is nil")
	}
	if w.Topic == "" {
		return nil, nil, oops.
			In("kafka").
			Tags("message-bus", "kafka", "producer", "configuration").
			Code(10308).
			Public("config_invalid").
			New("kafka.Writer.Topic is empty")
	}
	cleanup := func() { _ = w.Close() }
	return &KafkaWriter{W: w}, cleanup, nil
}

// NewKafkaReader builds a Reader from the full kafka.ReaderConfig.
func NewKafkaReader(cfg kafka.ReaderConfig) (*KafkaReader, func(), error) {
	if len(cfg.Brokers) == 0 {
		return nil, nil, oops.
			In("kafka").
			Tags("message-bus", "kafka", "consumer", "configuration").
			Code(10308).
			Public("config_invalid").
			New("kafka.ReaderConfig.Brokers is empty")
	}
	if cfg.Topic == "" {
		return nil, nil, oops.
			In("kafka").
			Tags("message-bus", "kafka", "consumer", "configuration").
			Code(10308).
			Public("config_invalid").
			With("brokers_count", len(cfg.Brokers)).
			New("kafka.ReaderConfig.Topic is empty")
	}
	r := kafka.NewReader(cfg)
	cleanup := func() { _ = r.Close() }
	return &KafkaReader{R: r}, cleanup, nil
}
