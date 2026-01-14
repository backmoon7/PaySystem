package mq

import (
	"log"

	"paysystem/internal/config"

	"github.com/IBM/sarama"
)

var KafkaProducer sarama.SyncProducer

// InitKafka 初始化 Kafka 生产者
func InitKafka(cfg *config.KafkaConfig) sarama.SyncProducer {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll // 等待所有副本确认
	kafkaConfig.Producer.Retry.Max = 3                    // 重试次数
	kafkaConfig.Producer.Return.Successes = true          // 返回成功消息

	producer, err := sarama.NewSyncProducer(cfg.Brokers, kafkaConfig)
	if err != nil {
		log.Fatalf("创建 Kafka 生产者失败: %v", err)
	}

	KafkaProducer = producer
	log.Println("Kafka 生产者创建成功")
	return producer
}

// SendMessage 发送消息到 Kafka
func SendMessage(topic, key, value string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(value),
	}

	_, _, err := KafkaProducer.SendMessage(msg)
	return err
}

// CloseKafka 关闭 Kafka 生产者
func CloseKafka() {
	if KafkaProducer != nil {
		KafkaProducer.Close()
	}
}
