package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
	"github.com/kopeio/kope/blobstore"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	s3Config := &aws.Config{}
	s3Config.Region = "us-east-1"
	s3Config.Credentials = aws.DefaultChainCredentials

	bucket := "meteor-galaxy-blobs"
	keyPrefix := "dev"
	s3 := s3.New(s3Config)
	blobStore := blobstore.NewS3BlobStore(s3, bucket, keyPrefix)
	blobServer := blobstore.NewBlobServer(blobStore)
	err := blobServer.ListenAndServe()
	if err != nil {
		glog.Fatalf("blobserver exited with error: %v", err)
	}
}
