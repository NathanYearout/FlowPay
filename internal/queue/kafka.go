package queue

import (
    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaClient struct {
    Producer *kafka.Producer
}

func NewKafkaClient(bootstrapServers string) (*KafkaClient, error) {
    p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": bootstrapServers})
    if err != nil {
        return nil, err
    }
    return &KafkaClient{Producer: p}, nil
}
