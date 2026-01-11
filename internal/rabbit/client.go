package rabbit

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/wb-go/wbf/zlog"
)

type Client struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	queue    string
}

type Rabbiter interface {
	NewRabbit(url, exchange, queue string) (*Client, error)
	Close()
	Publish(message []byte, delaySeconds int) error
	Consume(handler func([]byte) error) error
}

func NewRabbit(url, exchange, queue string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to connect to RabbitMQ")
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		zlog.Logger.Error().Err(err).Msg("failed to open RabbitMQ channel")
		return nil, err
	}

	client := &Client{
		conn:     conn,
		channel:  ch,
		exchange: exchange,
		queue:    queue,
	}

	args := amqp.Table{"x-delayed-type": "direct"}
	if err := ch.ExchangeDeclare(
		exchange,
		"x-delayed-message",
		true,
		false,
		false,
		false,
		args,
	); err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to declare exchange")
		return nil, err
	}

	if _, err := ch.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to declare queue")
		return nil, err
	}

	if err := ch.QueueBind(
		queue,
		"",
		exchange,
		false,
		nil,
	); err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to bind queue")
		return nil, err
	}

	zlog.Logger.Info().Msgf("RabbitMQ initialized (exchange=%s, queue=%s)", exchange, queue)

	return client, nil
}

func (c *Client) Close() {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	zlog.Logger.Info().Msg("RabbitMQ connection closed")
}

func (c *Client) Publish(message []byte, delaySeconds int) error {
	args := amqp.Table{}
	if delaySeconds > 0 {
		args["x-delay"] = int32(delaySeconds * 1000)
	}

	err := c.channel.Publish(
		c.exchange,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
			Timestamp:   time.Now(),
			Headers:     args,
		},
	)

	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to publish message to RabbitMQ")
	} else {
		zlog.Logger.Debug().Msgf("Message published to exchange=%s delay=%ds", c.exchange, delaySeconds)
	}
	return err
}

func (c *Client) Consume(handler func([]byte) error) error {
	msgs, err := c.channel.Consume(
		c.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to start consuming messages")
		return err
	}

	go func() {
		for d := range msgs {
			if err := handler(d.Body); err != nil {
				zlog.Logger.Warn().Msgf("failed to process message: %v", err)
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}()

	zlog.Logger.Info().Msgf("Started consuming from queue %s", c.queue)
	return nil
}
