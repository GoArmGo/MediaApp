package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client представляет собой клиент RabbitMQ
type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
	cfg     *config.Config
	logger  *slog.Logger
}

// NewClient создает и инициализирует новый клиент RabbitMQ
func NewClient(cfg *config.Config, logger *slog.Logger) (*Client, error) {
	start := time.Now()
	client := &Client{
		cfg:    cfg,
		logger: logger,
	}

	// Подключение к RabbitMQ
	conn, err := amqp.Dial(cfg.RabbitMQ.RabbitMQURL)
	if err != nil {
		logger.Error("failed to connect to RabbitMQ", "error", err)
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	client.conn = conn
	logger.Info("connected to RabbitMQ",
		"url", cfg.RabbitMQ.RabbitMQURL,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	// Открытие канала
	ch, err := conn.Channel()
	if err != nil {
		logger.Error("failed to open RabbitMQ channel", "error", err)
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}
	client.channel = ch
	logger.Info("RabbitMQ channel opened successfully")

	// Объявление очереди
	// Это идемпотентная операция: очередь будет создана, если ее нет,
	// и ничего не произойдет, если она уже существует.
	q, err := ch.QueueDeclare(
		cfg.RabbitMQ.RabbitMQQueueName, // name
		true,                           // durable - очередь будет сохраняться при перезапуске RabbitMQ
		false,                          // delete when unused
		false,                          // exclusive - только один потребитель
		false,                          // no-wait
		nil,                            // arguments
	)
	if err != nil {
		logger.Error("failed to declare queue", "queue", cfg.RabbitMQ.RabbitMQQueueName, "error", err)
		return nil, fmt.Errorf("failed to declare a queue: %v", err)
	}
	client.queue = q
	logger.Info("queue declared successfully",
		"queue", q.Name,
		"messages_in_queue", q.Messages,
	)

	return client, nil
}

// Close закрывает соединение и канал RabbitMQ
func (c *Client) Close() {
	start := time.Now()
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ channel", "error", err)
		} else {
			c.logger.Info("RabbitMQ channel closed")
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ connection", "error", err)
		} else {
			c.logger.Info("RabbitMQ connection closed", "duration_ms", time.Since(start).Milliseconds())
		}
	}
}

// PublishPhotoSearchRequest публикует сообщение о поиске фото в очередь RabbitMQ
func (c *Client) PublishPhotoSearchRequest(ctx context.Context, payload payloads.PhotoSearchPayload) error {
	// Маршалинг структуры payload в JSON
	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("failed to marshal payload", "error", err)
		return fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	err = c.channel.PublishWithContext(
		publishCtx,
		"",           // exchange
		c.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		c.logger.Error("failed to publish message", "queue", c.queue.Name, "error", err)
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	c.logger.Info("message published successfully",
		"queue", c.queue.Name,
		"payload", string(body),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// StartConsumingPhotoSearchRequests начинает потребление сообщений из очереди
// Этот метод реализует интерфейс ports.PhotoSearchConsumer
func (c *Client) StartConsumingPhotoSearchRequests(ctx context.Context, handler func(context.Context, payloads.PhotoSearchPayload) error) error {
	msgs, err := c.channel.Consume(
		c.queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		c.logger.Error("failed to register RabbitMQ consumer", "error", err)
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	c.logger.Info("consumer registered, waiting for messages", "queue", c.queue.Name)

	// Запускаем горутину для обработки сообщений
	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Warn("RabbitMQ channel closed, stopping consumer")
					return // Канал закрыт, выходим из горутины
				}

				var payload payloads.PhotoSearchPayload
				if err := json.Unmarshal(msg.Body, &payload); err != nil {
					c.logger.Error("failed to unmarshal message", "error", err, "body", string(msg.Body))
					// Если демаршалинг не удался
					// Отклоняем сообщение, но не возвращаем его в очередь (false, false)
					// чтобы не застрять в бесконечном цикле ошибок
					if err := msg.Nack(false, false); err != nil {
						c.logger.Error("failed to NACK message after unmarshal failure", "error", err)
					}
					continue // Переходим к следующему сообщению
				}

				c.logger.Info("received message from queue", "queue", c.queue.Name, "payload", payload)

				// Вызываем переданную функцию-обработчик
				if err := handler(ctx, payload); err != nil {
					c.logger.Error("error processing message", "error", err, "payload", payload)
					// Если обработка не удалась, возвращаем сообщение в очередь (requeue = true)
					if err := msg.Nack(false, true); err != nil {
						c.logger.Error("failed to NACK message after handler failure", "error", err)
					}
				} else {
					// Если обработка успешна, подтверждаем сообщение
					if err := msg.Ack(false); err != nil {
						c.logger.Error("failed to ACK message", "error", err)
					} else {
						c.logger.Info("message processed and ACKed", "payload", payload)
					}
				}
			case <-ctx.Done():
				// Контекст отменен, останавливаем потребление
				c.logger.Warn("context cancelled, stopping RabbitMQ consumer")
				return
			}
		}
	}()

	return nil
}
