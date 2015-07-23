package main

import (
	"flag"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
	"github.com/kopeio/kope/blobstore"
	"github.com/kopeio/kope/utils"
)

func readIfExists(path string) string {
	b, err := utils.ReadFileIfExists(path)
	if err != nil {
		glog.Fatalf("error reading file: %v %v", path, err)
	}
	return string(b)
}

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	s3Config := &aws.Config{}
	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	s3Config.Region = region
	creds := aws.DefaultChainCredentials
	awsAccessKey := readIfExists("/secrets/blobstore/aws-access-key")
	if awsAccessKey != "" {
		awsSecretKey := readIfExists("/secrets/blobstore/aws-secret-key")
		if awsSecretKey == "" {
			glog.Fatalf("aws-access-key found, but aws-secret-key not found")
		}

		// It is easy to introduce whitespace into a key by mistake
		awsAccessKey = strings.TrimSpace(awsAccessKey)
		awsSecretKey = strings.TrimSpace(awsSecretKey)

		glog.Info("Using credentials found in /secrets/blobstore accesskey=", awsAccessKey)
		creds = credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, "")
	}
	s3Config.Credentials = creds

	bucket := os.Getenv("S3_BUCKET")
	keyPrefix := os.Getenv("S3_PREFIX")
	if keyPrefix == "" {
		keyPrefix = "blobs"
	}
	glog.Info("Uploading to s3://", bucket, "/", keyPrefix)
	s3 := s3.New(s3Config)
	blobStore := blobstore.NewS3BlobStore(s3, bucket, keyPrefix)
	blobServer := blobstore.NewBlobServer(blobStore)
	err := blobServer.ListenAndServe()
	if err != nil {
		glog.Fatalf("blobserver exited with error: %v", err)
	}
}
