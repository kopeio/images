package blobstore

import (
	"errors"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
)

type S3BlobStore struct {
	s3        s3iface.S3API
	bucket    string
	keyPrefix string
}

func NewS3BlobStore(s3 s3iface.S3API, bucket string, keyPrefix string) *S3BlobStore {
	b := &S3BlobStore{}
	b.s3 = s3
	b.bucket = bucket
	b.keyPrefix = keyPrefix
	return b
}

func isValidComponent(s string) bool {
	for _, c := range s {
		if ('a' <= c) && (c <= 'z') {
		} else if ('A' <= c) && (c <= 'Z') {
		} else if ('0' <= c) && (c <= '9') {
		} else {
			return false
		}
	}
	return true
}

func (b *S3BlobStore) GetBlob(namespace string, blobId string) (Blob, error) {
	if !isValidComponent(namespace) || !isValidComponent(blobId) {
		glog.V(2).Info("Ignoring invalid namespace / blobId: ", namespace, "/", blobId)
		return nil, nil
	}
	bucket := b.bucket
	key := b.keyPrefix + "/" + namespace + "/" + blobId
	request := &s3.GetObjectInput{}
	request.Bucket = aws.String(bucket)
	request.Key = aws.String(key)
	glog.Info("Doing S3 request for ", bucket, "/", key)
	response, err := b.s3.GetObject(request)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			code := awsErr.Code()
			if code == "NoSuchKey" {
				return nil, nil
			}
			glog.V(2).Info("unknown AWS error", awsErr)
		}
		return nil, chained.Error(err, "error fetching object from s3")
	}
	blob := &S3Blob{}
	blob.response = response
	return blob, nil
}

func (b *S3BlobStore) PutBlob(namespace string, blobId string, r io.ReadSeeker, blobLength int64) error {
	if !isValidComponent(namespace) || !isValidComponent(blobId) {
		glog.V(2).Info("Ignoring invalid namespace / blobId: ", namespace, "/", blobId)
		return errors.New("Invalid namespace / blobId")
	}
	bucket := b.bucket
	key := b.keyPrefix + "/" + namespace + "/" + blobId
	request := &s3.PutObjectInput{}
	request.Bucket = aws.String(bucket)
	request.Key = aws.String(key)
	request.Body = r
	request.ContentLength = &blobLength
	request.ContentType = aws.String("application/octet-stream")
	glog.Info("Doing S3 PutObject for ", bucket, "/", key, " length=", blobLength)
	_, err := b.s3.PutObject(request)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			glog.Info("unknown AWS error", awsErr)
		}
		return chained.Error(err, "error writing object to S3")
	}
	// TODO: Check etag
	return nil
}

type S3Blob struct {
	response *s3.GetObjectOutput
}

func (b *S3Blob) Release() {
	err := b.response.Body.Close()
	if err != nil {
		glog.Warning("error closing S3 blob body", err)
	}
}

func (b *S3Blob) WriteTo(w io.Writer) error {
	_, err := io.Copy(w, b.response.Body)
	if err != nil {
		return chained.Error(err, "error copying S3 blob")
	}
	return nil
}
