package queue

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaClient struct {
	Producer *kafka.Producer
	Consumer *kafka.Consumer
}

func NewKafkaClient(bootstrapServers string) (*KafkaClient, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": bootstrapServers})
	if err != nil {
		return nil, fmt.Errorf("create producer: %w", err)
	}

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"group.id":          "flowpay-consumer",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("create consumer: %w", err)
	}

	return &KafkaClient{Producer: p, Consumer: c}, nil
}

func (k *KafkaClient) Publish(topic string, key string, payload interface{}) error {
	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)

	err = k.Producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          value,
	}, deliveryChan)
	if err != nil {
		return fmt.Errorf("produce message: %w", err)
	}

	e := <-deliveryChan
	msg := e.(*kafka.Message)
	if msg.TopicPartition.Error != nil {
		return fmt.Errorf("delivery failed: %w", msg.TopicPartition.Error)
	}

	log.Printf("published to %s [%d] at offset %v", topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset)
	return nil
}

func (k *KafkaClient) Subscribe(topics []string, handler func(key, value []byte)) error {
	if err := k.Consumer.SubscribeTopics(topics, nil); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	go func() {
		for {
			msg, err := k.Consumer.ReadMessage(-1)
			if err != nil {
				log.Printf("consumer error: %v", err)
				continue
			}
			handler(msg.Key, msg.Value)
		}
	}()

	return nil
}

func (k *KafkaClient) Close() {
	k.Producer.Close()
	k.Consumer.Close()
}
