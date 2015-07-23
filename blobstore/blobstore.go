package blobstore

import "io"

type BlobStore interface {
	GetBlob(namespace string, blobId string) (Blob, error)
	PutBlob(namespace string, blobId string, r io.ReadSeeker, blobLength int64) error
}

type Blob interface {
	Release()
	WriteTo(io.Writer) error
}
