package notification

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/streadway/amqp"
)

const (
	configRabbitConnectionString = "rabbit_host"
	configSocketQueue            = "socket_rabbit_queue"
	filesQueue                   = "filesQueue"
	permissionQueue              = "permissionsQueue"
)

type ObjectType = map[int32]string{
	0: "FILE",
	1: "PERMISSION",
	2: "NONE",
}

type Operation = map[int32]string{
	0: "ADD",
	1: "UPDATE",
	2: "DELETE",
	3: "NONE"
}

type listenerObject struct {
	objectID string
	objectType ObjectType
	operation OperationType
}

func handleError(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %s", msg, err)
	}
}

func connectRabbit() (conn *amqp.Connection) {
	conn, err := amqp.Dial(viper.GetString(configRabbitConnectionString))
	handleError(err, "Failed to connect to RabbitMQ")
	return conn
}

func openChannel(conn *amqp.Connection) (ch *amqp.Channel) {
	ch, err := conn.Channel()
	handleError(err, "Failed to open a channel")
	return ch
}

func declareExchange(ch *amqp.Channel) {
	err := ch.ExchangeDeclare("events", "topic", true, false, false, false, nil)
	handleError(err, "Failed to declare exchange")
}

func declareQueue(ch *amqp.Channel, name string) (q amqp.Queue) {
	q, err := ch.QueueDeclare(
		name,  // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	handleError(err, "Failed to declare a queue")

	return q
}

func activateConsumer(ch *amqp.Channel, queueName string) <-chan amqp.Delivery {
	msgs, err := ch.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	handleError(err, "Failed to register a consumer")

	return msgs
}

func SetUp() {
	conn := connectRabbit()
	defer conn.Close()

	ch := openChannel(conn)
	defer ch.Close()

	msgs := activateConsumer(ch, filesQueue)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("Received a message: %s", d.Body)
			d.Ack(false)
		}
	}()
	<-forever
}
