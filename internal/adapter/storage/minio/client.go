package minio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"

	appconfig "github.com/GoArmGo/MediaApp/internal/config"
)

// Client представляет собой клиент для взаимодействия с MinIO (S3-совместимым хранилищем)
type Client struct {
	s3Client   *s3.Client
	uploader   *manager.Uploader
	bucketName string
	logger     *slog.Logger
}

// NewMinioClient создает и инициализирует новый MinIO Client, используя переданную конфигурацию
func NewMinioClient(cfg *appconfig.Config, logger *slog.Logger) (*Client, error) {
	minioAccessKey := cfg.MinioAccessKeyID
	minioSecretKey := cfg.MinioSecretAccessKey
	minioBucketName := cfg.MinioBucketName
	minioEndpoint := cfg.MinioEndpoint
	minioUseSSL := cfg.MinioUseSSL
	minioRegion := cfg.MinioRegion

	if minioAccessKey == "" || minioSecretKey == "" || minioBucketName == "" || minioEndpoint == "" || minioRegion == "" {
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
	)
	if err != nil {
		logger.Error("failed to load AWS config for MinIO", "error", err)
		return nil, fmt.Errorf("failed to load AWS config for MinIO: %w", err)
	}

	s3Client := s3.NewFromConfig(cfgAws, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Создаем uploader без функциональных опций
	uploader := manager.NewUploader(s3Client)

	// Настраиваем параметры напрямую через поля структуры
	// Примеры настроек :
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
		logger.Warn("bucket not found, creating...", "bucket", minioBucketName)

		_, createErr := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(minioBucketName),
			// Для MinIO может потребоваться явное указание региона
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(minioRegion),
			},
		})

		if createErr != nil {
			logger.Error("failed to create bucket", "bucket", minioBucketName, "error", createErr)
			return nil, fmt.Errorf("failed to create bucket '%s': %w", minioBucketName, createErr)
		}

		// Ждем пока бакет станет доступен
		waiter := s3.NewBucketExistsWaiter(s3Client)
		if err := waiter.Wait(context.TODO(), &s3.HeadBucketInput{
			Bucket: aws.String(minioBucketName),
		}, 30*time.Second); err != nil {
			logger.Error("failed waiting for bucket to be created", "bucket", minioBucketName, "error", err)
			return nil, fmt.Errorf("failed waiting for bucket '%s' to be created: %w", minioBucketName, err)
		}

		logger.Info("bucket created successfully", "bucket", minioBucketName)
	} else {
		logger.Info("bucket already exists", "bucket", minioBucketName)
	}

	return &Client{
		s3Client:   s3Client,
		uploader:   uploader,
		bucketName: minioBucketName,
		logger:     logger,
	}, nil
}

// UploadFile загружает файл в указанный бакет MinIO
func (c *Client) UploadFile(ctx context.Context, objectKey string, fileContent io.Reader, contentType string) (string, error) {
	start := time.Now()

	uploadOutput, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucketName),
		Key:         aws.String(objectKey),
		Body:        fileContent,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		c.logger.Error("failed to upload file",
			"bucket", c.bucketName,
			"object", objectKey,
			"error", err,
		)
		return "", fmt.Errorf("failed to upload file %s to bucket %s using multipart upload: %w", objectKey,
			c.bucketName, err)
	}

	duration := time.Since(start)
	c.logger.Info("file uploaded successfully",
		"bucket", c.bucketName,
		"object", objectKey,
		"location", uploadOutput.Location,
		"duration_ms", duration.Milliseconds(),
	)

	return fmt.Sprintf("%s/%s/%s", "http://localhost:9000", c.bucketName, objectKey), nil
}

// GetFile получает содержимое файла из MinIO
func (c *Client) GetFile(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	start := time.Now()
	output, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		c.logger.Error("failed to get file", "bucket", c.bucketName, "object", objectKey, "error", err)
		return nil, fmt.Errorf("failed to get file %s from bucket %s: %w", objectKey, c.bucketName, err)
	}
	c.logger.Info("file fetched successfully",
		"bucket", c.bucketName,
		"object", objectKey,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return output.Body, nil
}

// DeleteFile удаляет файл из MinIO
func (c *Client) DeleteFile(ctx context.Context, objectKey string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		c.logger.Error("failed to delete file", "bucket", c.bucketName, "object", objectKey, "error", err)
		return fmt.Errorf("failed to delete file %s from bucket %s: %w", objectKey, c.bucketName, err)
	}
	c.logger.Info("file deleted successfully", "bucket", c.bucketName, "object", objectKey)
	return nil
}
