package ports

import (
	"context"

	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"
)

// PhotoSearchPublisher определяет методы для публикации сообщений о поиске фото.
// Этот интерфейс будет использоваться обработчиком HTTP-запросов.
type PhotoSearchPublisher interface {
	PublishPhotoSearchRequest(ctx context.Context, payload payloads.PhotoSearchPayload) error
}

// PhotoSearchConsumer определяет методы для потребления сообщений о поиске фото.
// Он будет использоваться воркером для получения задач из очереди.
type PhotoSearchConsumer interface {
	// StartConsumingPhotoSearchRequests начинает прослушивание очереди для сообщений о поиске фото.
	// Он принимает функцию-обработчик, которая будет вызываться для каждого полученного сообщения.
	StartConsumingPhotoSearchRequests(ctx context.Context, handler func(context.Context, payloads.PhotoSearchPayload) error) error
}
