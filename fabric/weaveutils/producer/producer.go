package producer

import (
	"fmt"
	"os"
	"strconv"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var (
	kafkabroker1 = os.Getenv("KAFKA_BROKER1")
	kafkabroker2 = os.Getenv("KAFKA_BROKER2")
	kafkabroker3 = os.Getenv("KAFKA_BROKER3")
)

func Config() *kafka.Producer {
	brokers := kafkabroker1 + "," + kafkabroker2 + "," + kafkabroker3

	batchSize, _ := strconv.Atoi(os.Getenv("KAFKA_BATCHSIZE"))
	linger, _ := strconv.Atoi(os.Getenv("KAFKA_LINGER"))

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"acks":              "all",
		"batch.size":        batchSize,
		"linger.ms":         linger,
	})
	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		panic(err)
	}

	return p
}
