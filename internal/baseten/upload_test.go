package baseten

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
)

type fakeS3UploadClient struct {
	bucket string
	key    string
	body   string
}

func (client *fakeS3UploadClient) UploadObject(_ context.Context, input *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	if input.Bucket != nil {
		client.bucket = *input.Bucket
	}

	if input.Key != nil {
		client.key = *input.Key
	}

	if input.Body != nil {
		body, err := io.ReadAll(input.Body)
		if err != nil {
			return nil, err
		}

		client.body = string(body)
	}

	return &transfermanager.UploadObjectOutput{}, nil
}

func TestUploadModelArchiveWithClient(t *testing.T) {
	uploader := &fakeS3UploadClient{}
	bucket := "baseten-upload"
	key := "archives/model.tar.gz"
	body := bytes.NewReader([]byte("archive bytes"))

	err := uploadModelArchiveWithClient(context.Background(), uploader, &bucket, &key, body)
	if err != nil {
		t.Fatalf("uploadModelArchiveWithClient: %v", err)
	}

	if uploader.bucket != "baseten-upload" {
		t.Fatalf("bucket = %q, want baseten-upload", uploader.bucket)
	}

	if uploader.key != "archives/model.tar.gz" {
		t.Fatalf("key = %q, want archives/model.tar.gz", uploader.key)
	}

	if uploader.body != "archive bytes" {
		t.Fatalf("body = %q, want archive bytes", uploader.body)
	}
}

func TestUploadModelArchiveWithClientRejectsMissingDestination(t *testing.T) {
	uploader := &fakeS3UploadClient{}
	key := "archives/model.tar.gz"
	body := bytes.NewReader([]byte("archive bytes"))

	if err := uploadModelArchiveWithClient(context.Background(), uploader, nil, &key, body); err == nil {
		t.Fatal("uploadModelArchiveWithClient accepted a nil bucket")
	}

	bucket := "baseten-upload"
	if err := uploadModelArchiveWithClient(context.Background(), uploader, &bucket, nil, body); err == nil {
		t.Fatal("uploadModelArchiveWithClient accepted a nil key")
	}
}

func TestUploadModelArchiveRejectsMissingCredentials(t *testing.T) {
	bucket := "baseten-upload"
	key := "archives/model.tar.gz"
	region := "us-west-2"
	body := bytes.NewReader([]byte("archive bytes"))

	err := UploadModelArchive(context.Background(), PrepareModelUploadResponse{
		S3Bucket: &bucket,
		S3Key:    &key,
		S3Region: &region,
	}, body)
	if err == nil {
		t.Fatal("UploadModelArchive accepted missing credentials")
	}
}
