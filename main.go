package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	defaultGPT *catGPT
	isReady    atomic.Bool
)

func main() {
	bucketName := os.Getenv("CATGPT_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("you must supply bucket name (via CATGPT_BUCKET_NAME=BUCKET env variable)")
	}

	listenPublic := ":8080"
	if lp := os.Getenv("CATGPT_LISTEN_PUBLIC"); lp != "" {
		listenPublic = lp
	}

	// This listener should not be exposed to the internet.
	listenPrivate := ":9090"
	if lp := os.Getenv("CATGPT_LISTEN_PRIVATE"); lp != "" {
		listenPrivate = lp
	}

	c, err := newS3Client(bucketName)
	if err != nil {
		log.Fatal("failed to initialize s3 client", err)
	}
	defaultGPT = &catGPT{
		bucket: bucketName,
		client: c,
	}
	serve(context.Background(), listenPublic, listenPrivate)
}

func newS3Client(bucket string) (*s3.Client, error) {
	// Creating a custom endpoint resolver for returning the correct URL for S3 storage in the ru-central1 region
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID && region == "ru-central1" {
			return aws.Endpoint{
				PartitionID:   "yc",
				URL:           "https://storage.yandexcloud.net",
				SigningRegion: "ru-central1",
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
	})

	// Loading configuration from ~/.aws/*
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		return nil, err
	}

	// Creating an S3 client
	client := s3.NewFromConfig(cfg)
	return client, nil
}
