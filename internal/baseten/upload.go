package baseten

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3PutObjectClient interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func UploadModelArchive(ctx context.Context, upload PrepareModelUploadResponse, body io.Reader) error {
	if upload.Credentials == nil {
		return errors.New("missing upload credentials")
	}

	if upload.S3Region == nil || *upload.S3Region == "" {
		return errors.New("missing upload S3 region")
	}

	awsConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(*upload.S3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			upload.Credentials.AccessKeyID,
			upload.Credentials.SecretAccessKey,
			upload.Credentials.SessionToken,
		)),
	)
	if err != nil {
		return fmt.Errorf("load upload AWS config: %w", err)
	}

	uploader := s3.NewFromConfig(awsConfig)
	return uploadModelArchiveWithClient(ctx, uploader, upload.S3Bucket, upload.S3Key, body)
}

func uploadModelArchiveWithClient(ctx context.Context, uploader s3PutObjectClient, bucket *string, key *string, body io.Reader) error {
	if bucket == nil || *bucket == "" {
		return errors.New("missing upload S3 bucket")
	}

	if key == nil || *key == "" {
		return errors.New("missing upload S3 key")
	}

	_, err := uploader.PutObject(ctx, &s3.PutObjectInput{
		Bucket: bucket,
		Key:    key,
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("upload model archive: %w", err)
	}

	return nil
}
