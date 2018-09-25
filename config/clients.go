package config

import (
	"github.com/prometheus/common/model"
	"github.com/yuriadams/prometheus-kafka-adapter/kafka"
)

// Writer represents the interface that each client must implement write function
type Writer interface {
	Write(samples model.Samples) error
}

// BuildClient returns kafka client with writer function
func BuildClient() Writer {
	cfg := GetConfig()

	if cfg.KafkaBrokers != "" {
		return kafka.NewClient(
			cfg.KafkaBrokers,
			cfg.KafkaVerbose,
			cfg.KafkaCertFile,
			cfg.KafkaKeyFile,
			cfg.KafkaCaFile,
			cfg.KafkaVerifySsl,
		)
	}
	return nil
}

//
// func CloseClient() {
// 	defer func() {
// 		if err := kafka.Close(); err != nil {
// 			log.Println("Failed to close client", err)
// 		}
// 	}()
// }
