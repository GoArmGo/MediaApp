package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config" // Ваш пакет конфигурации
	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"

	amqp "github.com/rabbitmq/amqp091-go" // Пакет RabbitMQ
)

// Client представляет собой клиент RabbitMQ
type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
	cfg     *config.Config
}

// NewClient создает и инициализирует новый клиент RabbitMQ
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		cfg: cfg,
	}

	// Подключение к RabbitMQ
	conn, err := amqp.Dial(cfg.RabbitMQ.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	client.conn = conn
	log.Println("Successfully connected to RabbitMQ!")

	// Открытие канала
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}
	client.channel = ch
	log.Println("Channel opened successfully.")

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
		return nil, fmt.Errorf("failed to declare a queue: %v", err)
	}
	client.queue = q
	log.Printf("Queue '%s' declared successfully. Messages in queue: %d", q.Name, q.Messages)

	return client, nil
}

// Close закрывает соединение и канал RabbitMQ
func (c *Client) Close() {
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		} else {
			log.Println("RabbitMQ channel closed.")
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing RabbitMQ connection: %v", err)
		} else {
			log.Println("RabbitMQ connection closed.")
		}
	}
}

// PublishPhotoSearchRequest публикует сообщение о поиске фото в очередь RabbitMQ.
// Этот метод теперь соответствует интерфейсу ports.PhotoSearchPublisher.
func (c *Client) PublishPhotoSearchRequest(ctx context.Context, payload payloads.PhotoSearchPayload) error {
	// Маршалинг структуры payload в JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = c.channel.PublishWithContext(
		publishCtx,
		"",           // exchange
		c.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json", // Указываем, что содержимое - JSON
			Body:        body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	log.Printf("Message published to queue '%s': %s", c.queue.Name, string(body))
	return nil
}

// StartConsumingPhotoSearchRequests начинает потребление сообщений из очереди.
// Этот метод реализует интерфейс ports.PhotoSearchConsumer.
func (c *Client) StartConsumingPhotoSearchRequests(ctx context.Context, handler func(context.Context, payloads.PhotoSearchPayload) error) error {
	msgs, err := c.channel.Consume(
		c.queue.Name, // queue
		"",           // consumer
		false,        // auto-ack (мы будем подтверждать вручную)
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	log.Printf("Consumer registered for queue '%s'. Waiting for messages...", c.queue.Name)

	// Запускаем горутину для обработки сообщений
	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					log.Println("RabbitMQ channel closed, stopping consumer.")
					return // Канал закрыт, выходим из горутины
				}

				var payload payloads.PhotoSearchPayload
				if err := json.Unmarshal(msg.Body, &payload); err != nil {
					log.Printf("Error unmarshalling message: %v, body: %s", err, string(msg.Body))
					// Если демаршалинг не удался, это, вероятно, плохой формат сообщения.
					// Отклоняем сообщение, но не возвращаем его в очередь (false, false)
					// чтобы не застрять в бесконечном цикле ошибок.
					if err := msg.Nack(false, false); err != nil {
						log.Printf("Error NACKing message after unmarshal failure: %v", err)
					}
					continue // Переходим к следующему сообщению
				}

				log.Printf("Received message from queue: %+v", payload)

				// Вызываем переданную функцию-обработчик
				if err := handler(ctx, payload); err != nil {
					log.Printf("Error processing message: %v, payload: %+v", err, payload)
					// Если обработка не удалась, возвращаем сообщение в очередь (requeue = true)
					if err := msg.Nack(false, true); err != nil {
						log.Printf("Error NACKing message after processing failure: %v", err)
					}
				} else {
					// Если обработка успешна, подтверждаем сообщение
					if err := msg.Ack(false); err != nil {
						log.Printf("Error ACKing message: %v", err)
					}
					log.Printf("Message processed and ACKed: %+v", payload)
				}
			case <-ctx.Done():
				// Контекст отменен, останавливаем потребление
				log.Println("Context cancelled, stopping RabbitMQ consumer.")
				return
			}
		}
	}()

	return nil
}
