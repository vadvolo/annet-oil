package logging

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"annet-oil/internal/config"
)

type S3Uploader struct {
	client   *s3.Client
	bucket   string
	prefix   string
	logPath  string
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewS3Uploader(cfg config.LoggingConfig) (*S3Uploader, error) {
	if !cfg.S3.Enabled || cfg.S3.Bucket == "" {
		return nil, nil
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Uploader{
		client:   s3.NewFromConfig(awsCfg),
		bucket:   cfg.S3.Bucket,
		prefix:   cfg.S3.Prefix,
		logPath:  cfg.Output,
		interval: time.Hour,
		stopCh:   make(chan struct{}),
	}, nil
}

func (u *S3Uploader) Start() {
	if u == nil {
		return
	}

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		ticker := time.NewTicker(u.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := u.uploadAndRotate(); err != nil {
					Error("S3 upload failed", "error", err)
				}
			case <-u.stopCh:
				u.uploadAndRotate()
				return
			}
		}
	}()
}

func (u *S3Uploader) Stop() {
	if u == nil {
		return
	}
	close(u.stopCh)
	u.wg.Wait()
}

func (u *S3Uploader) uploadAndRotate() error {
	if u.logPath == "" {
		return nil
	}

	info, err := os.Stat(u.logPath)
	if err != nil || info.Size() == 0 {
		return nil
	}

	rotatedPath := fmt.Sprintf("%s.%s", u.logPath, time.Now().Format("20060102-150405"))
	if err := os.Rename(u.logPath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate log: %w", err)
	}

	data, err := os.ReadFile(rotatedPath)
	if err != nil {
		return fmt.Errorf("failed to read rotated log: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return fmt.Errorf("failed to compress log: %w", err)
	}
	gz.Close()

	key := fmt.Sprintf("%s%s/%s.gz",
		u.prefix,
		time.Now().Format("2006/01/02"),
		filepath.Base(rotatedPath),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(u.bucket),
		Key:             aws.String(key),
		Body:            io.NopCloser(&buf),
		ContentType:     aws.String("application/gzip"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	Info("Log uploaded to S3", "bucket", u.bucket, "key", key)
	os.Remove(rotatedPath)

	return nil
}
