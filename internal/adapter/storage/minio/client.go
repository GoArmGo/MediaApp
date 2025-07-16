// internal/adapter/storage/minio/client.go
package minio

import (
	"context"
	"fmt"
	"io"
	"log"
	"time" // <-- ДОБАВЬТЕ ЭТОТ ИМПОРТ ДЛЯ time.Second

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types" // <-- ДОБАВЬТЕ ЭТОТ ВАЖНЫЙ ИМПОРТ ДЛЯ ТИПОВ S3

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"

	appconfig "github.com/GoArmGo/MediaApp/internal/config"
)

// Client представляет собой клиент для взаимодействия с MinIO (S3-совместимым хранилищем).
type Client struct {
	s3Client   *s3.Client
	uploader   *manager.Uploader
	bucketName string
}

// NewMinioClient создает и инициализирует новый MinIO Client, используя переданную конфигурацию.
func NewMinioClient(cfg *appconfig.Config) (*Client, error) {
	minioAccessKey := cfg.MinioAccessKeyID
	minioSecretKey := cfg.MinioSecretAccessKey
	minioBucketName := cfg.MinioBucketName
	minioEndpoint := cfg.MinioEndpoint
	minioUseSSL := cfg.MinioUseSSL
	minioRegion := cfg.MinioRegion // <-- ТЕПЕРЬ cfg.MinioRegion ДОСТУПЕН

	if minioAccessKey == "" || minioSecretKey == "" || minioBucketName == "" || minioEndpoint == "" || minioRegion == "" { // <-- Добавил проверку на minioRegion
		return nil, fmt.Errorf("MinIO credentials (MINIO_ACCESS_KEY_ID, MINIO_SECRET_ACCESS_KEY, MINIO_BUCKET_NAME, MINIO_ENDPOINT, MINIO_REGION) must be set in environment variables")
	}

	var fullMinioEndpointURL string
	if minioUseSSL {
		fullMinioEndpointURL = fmt.Sprintf("https://%s", minioEndpoint)
	} else {
		fullMinioEndpointURL = fmt.Sprintf("http://%s", minioEndpoint)
	}

	cfgAws, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(minioAccessKey, minioSecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:    fullMinioEndpointURL,
					Source: aws.EndpointSourceCustom,
				}, nil
			})),
		// Убираем логгирование для production, оставляем только для отладки
		// awsconfig.WithClientLogMode(aws.LogRequest|aws.LogResponse), // <-- Закомментировано по вашему запросу
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for MinIO: %w", err)
	}

	s3Client := s3.NewFromConfig(cfgAws, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// ----------------------------
	// АЛЬТЕРНАТИВНОЕ РЕШЕНИЕ (пункт #4)
	// Создаем uploader без функциональных опций
	uploader := manager.NewUploader(s3Client)

	// Настраиваем параметры напрямую через поля структуры
	// Примеры настроек (раскомментируйте при необходимости):
	// uploader.PartSize = 64 * 1024 * 1024 // 64MB per part
	// uploader.Concurrency = 5             // 5 параллельных загрузок
	// uploader.LeavePartsOnError = true    // Не удалять части при ошибке
	// ----------------------------

	// Проверяем существование бакета
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(minioBucketName),
	})

	if err != nil {
		log.Printf("Bucket '%s' not found, creating...", minioBucketName)

		_, createErr := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(minioBucketName),
			// Для MinIO может потребоваться явное указание региона
			CreateBucketConfiguration: &types.CreateBucketConfiguration{ // <-- ИСПОЛЬЗУЕМ types.CreateBucketConfiguration
				LocationConstraint: types.BucketLocationConstraint(minioRegion), // <-- ИСПОЛЬЗУЕМ types.BucketLocationConstraint и minioRegion
			},
		})

		if createErr != nil {
			return nil, fmt.Errorf("failed to create bucket '%s': %w", minioBucketName, createErr)
		}

		// Ждем пока бакет станет доступен
		waiter := s3.NewBucketExistsWaiter(s3Client)
		if err := waiter.Wait(context.TODO(), &s3.HeadBucketInput{
			Bucket: aws.String(minioBucketName),
		}, 30*time.Second); err != nil {
			return nil, fmt.Errorf("failed waiting for bucket '%s' to be created: %w", minioBucketName, err)
		}

		log.Printf("Bucket '%s' created successfully", minioBucketName)
	} else {
		log.Printf("Bucket '%s' already exists", minioBucketName)
	}

	return &Client{
		s3Client:   s3Client,
		uploader:   uploader,
		bucketName: minioBucketName,
	}, nil
}

// UploadFile загружает файл в указанный бакет MinIO.
// ... (остальной код остается без изменений)
func (c *Client) UploadFile(ctx context.Context, objectKey string, fileContent io.Reader, contentType string) (string, error) {
	uploadOutput, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucketName),
		Key:         aws.String(objectKey),
		Body:        fileContent,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file %s to bucket %s using multipart upload: %w", objectKey, c.bucketName, err)
	}

	log.Printf("MinIO: Файл '%s' успешно загружен. Location: %s", objectKey, uploadOutput.Location)
	return fmt.Sprintf("%s/%s/%s", "http://localhost:9000", c.bucketName, objectKey), nil
}

// GetFile получает содержимое файла из MinIO.
func (c *Client) GetFile(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	output, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s from bucket %s: %w", objectKey, c.bucketName, err)
	}
	return output.Body, nil
}

// DeleteFile удаляет файл из MinIO.
// ... (остальной код остается без изменений)
func (c *Client) DeleteFile(ctx context.Context, objectKey string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file %s from bucket %s: %w", objectKey, c.bucketName, err)
	}
	return nil
}
