package consumer

import (
	"fmt"
	"os"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var (
	kafkabroker1 = os.Getenv("KAFKA_BROKER1")
	kafkabroker2 = os.Getenv("KAFKA_BROKER2")
	kafkabroker3 = os.Getenv("KAFKA_BROKER3")
)

func Config() *kafka.Consumer {
	brokers := kafkabroker1 + "," + kafkabroker2 + "," + kafkabroker3

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "ackGroup",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		fmt.Printf("Failed to create consumer: %s", err)
		panic(err)
	}
	return c
}
