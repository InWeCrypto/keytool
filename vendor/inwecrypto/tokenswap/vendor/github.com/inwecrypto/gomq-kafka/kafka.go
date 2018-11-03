package gomq

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/dynamicgo/config"
	"github.com/dynamicgo/slf4go"
	mq "github.com/inwecrypto/gomq"
)

var logger = slf4go.Get("mq")

// AliyunProducer aliyun mq client using kafka protocol
type AliyunProducer struct {
	producer sarama.SyncProducer
}

// NewAliyunProducer create new aliyun mq client
func NewAliyunProducer(cnf *config.Config) (*AliyunProducer, error) {

	logger.DebugF("create aliyun kafka producer with config:\n%s", cnf)

	kafkaConfig := sarama.NewConfig()

	kafkaConfig.Metadata.Retry.Max = 10000000000

	kafkaConfig.Net.SASL.Enable = true

	kafkaConfig.Net.SASL.User = cnf.GetString("aliyun.kafka.user", "xxxx")
	// The aliyun kafka use SecretKey last 10 chars as password
	kafkaConfig.Net.SASL.Password = cnf.GetString("aliyun.kafka.password", "xxxx")

	kafkaConfig.Net.SASL.Handshake = true

	certBytes, err := ioutil.ReadFile(cnf.GetString("aliyun.kafka.cert", "/etc/inwecrypto/kafka.cert"))

	if err != nil {
		return nil, err
	}

	clientCertPool := x509.NewCertPool()
	ok := clientCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, fmt.Errorf("kafka producer failed to parse root certificate")
	}

	kafkaConfig.Net.TLS.Config = &tls.Config{
		//Certificates:       []tls.Certificate{},
		RootCAs:            clientCertPool,
		InsecureSkipVerify: true,
	}

	kafkaConfig.Net.TLS.Enable = true
	kafkaConfig.Producer.Return.Successes = true

	if err = kafkaConfig.Validate(); err != nil {
		return nil, fmt.Errorf("kafka producer config invalidate. err: %v", err)
	}

	var servers = []string{
		"xxxxx",
	}

	cnf.GetObject("aliyun.kafka.servers", &servers)

	producer, err := sarama.NewSyncProducer(servers, kafkaConfig)

	if err != nil {
		return nil, fmt.Errorf("kafka producer create fail. err: %v", err)
	}

	return &AliyunProducer{
		producer: producer,
	}, err
}

// Produce produce new kafka message
func (producer *AliyunProducer) Produce(topic string, key []byte, content interface{}) error {

	data, err := json.Marshal(content)

	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = producer.producer.SendMessage(msg)

	if err != nil {
		return fmt.Errorf(
			"Kafka send message error. topic: %v. key: %v. content: %v\n\t%s",
			topic, hex.EncodeToString(key), content, err,
		)

	}

	return nil
}

// Batch .
func (producer *AliyunProducer) Batch(messages []*mq.BatchMessage) error {

	var msgs []*sarama.ProducerMessage

	for _, message := range messages {
		data, err := json.Marshal(message.Content)

		if err != nil {
			return err
		}

		msgs = append(msgs, &sarama.ProducerMessage{
			Topic: message.Topic,
			Key:   sarama.ByteEncoder(message.Key),
			Value: sarama.ByteEncoder(data),
		})
	}

	err := producer.producer.SendMessages(msgs)

	if err != nil {
		return fmt.Errorf(
			"Kafka send message error. %d", len(messages),
		)

	}

	return nil
}

// AliyunConsumer .
type AliyunConsumer struct {
	consumer *cluster.Consumer
	messages chan mq.Message
}

type kafkaConsumerMessageWraper struct {
	message *sarama.ConsumerMessage
}

func (wraper *kafkaConsumerMessageWraper) Key() []byte {
	return wraper.message.Key
}
func (wraper *kafkaConsumerMessageWraper) Topic() string {
	return wraper.message.Topic
}
func (wraper *kafkaConsumerMessageWraper) Value() []byte {
	return wraper.message.Value
}

func (wraper *kafkaConsumerMessageWraper) Offset() int64 {
	return wraper.message.Offset
}

// NewAliyunConsumer create new aliyun mq consumer
func NewAliyunConsumer(cnf *config.Config) (*AliyunConsumer, error) {
	logger.DebugF("create aliyun kafka producer with config:\n%s", cnf)

	clusterCfg := cluster.NewConfig()

	clusterCfg.Metadata.Retry.Max = 10000000000

	clusterCfg.Net.SASL.Enable = true
	clusterCfg.Net.SASL.User = cnf.GetString("aliyun.kafka.user", "xxxx")
	// The aliyun kafka use SecretKey last 10 chars as password
	clusterCfg.Net.SASL.Password = cnf.GetString("aliyun.kafka.password", "xxxx")
	clusterCfg.Net.SASL.Handshake = true

	certBytes, err := ioutil.ReadFile(cnf.GetString("aliyun.kafka.cert", "/etc/inwecrypto/kafka.cert"))

	if err != nil {
		return nil, err
	}

	clientCertPool := x509.NewCertPool()
	ok := clientCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, fmt.Errorf("kafka producer failed to parse root certificate")
	}

	clusterCfg.Net.TLS.Config = &tls.Config{
		//Certificates:       []tls.Certificate{},
		RootCAs:            clientCertPool,
		InsecureSkipVerify: true,
	}

	clusterCfg.Net.TLS.Enable = true
	clusterCfg.Consumer.Return.Errors = true
	clusterCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	clusterCfg.Group.Return.Notifications = true

	clusterCfg.Version = sarama.V0_10_0_0
	if err = clusterCfg.Validate(); err != nil {
		return nil, fmt.Errorf("kafka producer config invalidate. err: %v", err)
	}

	var servers = []string{
		"xxxxx",
	}

	cnf.GetObject("aliyun.kafka.servers", &servers)

	var topics = []string{
		"xxxxx",
	}

	cnf.GetObject("aliyun.kafka.topics", &topics)

	logger.Debug(topics, cnf.GetString("aliyun.kafka.consumer", "xxxxx"))

	consumer, err := cluster.NewConsumer(servers, cnf.GetString("aliyun.kafka.consumer", "xxxxx"), topics, clusterCfg)

	if err != nil {
		return nil, fmt.Errorf("kafka consumer create fail. err: %v", err)
	}

	aliyun := &AliyunConsumer{
		consumer: consumer,
		messages: make(chan mq.Message),
	}

	go aliyun.run()

	return aliyun, nil
}

func (consumer *AliyunConsumer) run() {
	for {
		select {
		case msg, more := <-consumer.consumer.Messages():
			if more {
				consumer.messages <- &kafkaConsumerMessageWraper{msg}
			} else {
				close(consumer.messages)
			}
		case notify, more := <-consumer.consumer.Notifications():
			if more {
				logger.DebugF("Kafka consumer rebalance: %v", notify)
			}
		}
	}
}

// Close .
func (consumer *AliyunConsumer) Close() {
	consumer.consumer.Close()
}

// Messages return message chan
func (consumer *AliyunConsumer) Messages() <-chan mq.Message {
	return consumer.messages
}

// Errors return error chan
func (consumer *AliyunConsumer) Errors() <-chan error {
	return consumer.consumer.Errors()
}

// Commit commit current handle message as consumed
func (consumer *AliyunConsumer) Commit(message mq.Message) {
	consumer.consumer.MarkOffset(message.(*kafkaConsumerMessageWraper).message, "")
}
