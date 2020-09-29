package notification

import (
	"bytes"
	"log"
	"time"

	"github.com/spf13/viper"
	"github.com/streadway/amqp"
)

const (
	configRabbitConnectionString = "rabbit_host"
	configSocketQueue            = "socket_rabbit_queue"
	filesQueue                   = "filesQueue"
	permissionQueue              = "permissionsQueue"
)

func handleError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func declareQueue(ch *amqp.Channel, name string) (q amqp.Queue) {
	q, err := ch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	handleError(err, "Failed to declare a queue")

	return q
}

func SetUp() {
	conn, err := amqp.Dial(viper.GetString(configRabbitConnectionString))
	handleError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	handleError(err, "Failed to open a channel")
	defer ch.Close()

	fileQ := declareQueue(ch, viper.GetString(filesQueue))           // listener-service file queue
	// permssionQ := declareQueue(ch, viper.GetString(permissionQueue)) // listener-service file queue
	// ssq:= declareQueue(ch, viper.GetString(configSocketQueue)) // socket-service queue

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	handleError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		fileQ.Name, // queue
		"",         // consumer
		false,      // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	handleError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			dot_count := bytes.Count(d.Body, []byte("."))
			t := time.Duration(dot_count)
			time.Sleep(t * time.Second)
			log.Printf("Done")
			d.Ack(false)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
