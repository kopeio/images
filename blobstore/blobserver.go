package blobstore

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
)

type BlobServer struct {
	blobStore BlobStore
	tempDir   string
}

func NewBlobServer(blobStore BlobStore) *BlobServer {
	b := &BlobServer{}
	b.blobStore = blobStore
	b.tempDir = os.TempDir()
	return b
}

func (b *BlobServer) blobHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if len(path) != 0 && path[0] == '/' {
		// Skip leading slash
		path = path[1:]
	}
	urlTokens := strings.Split(path, "/")
	if urlTokens[0] == "blob" {
		if len(urlTokens) == 3 {
			namespace := urlTokens[1]
			blobId := urlTokens[2]
			if r.Method == "GET" {
				b.getBlob(w, r, namespace, blobId)
				return
			}
			if r.Method == "PUT" {
				err := b.putBlob(w, r, namespace, blobId)
				if err != nil {
					glog.Warning("putBlob returned error", err)
					http.Error(w, "Internal error", http.StatusInternalServerError)
				}
				return
			}
		}
	}
	http.NotFound(w, r)
}

func (b *BlobServer) getBlob(w http.ResponseWriter, r *http.Request, namespace string, blobId string) {
	blob, err := b.blobStore.GetBlob(namespace, blobId)
	if err != nil {
		glog.Warning("Error reading blob", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if blob == nil {
		http.NotFound(w, r)
		return
	}
	defer blob.Release()
	err = blob.WriteTo(w)
	if err != nil {
		glog.Warning("Error copying blob", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

func (b *BlobServer) putBlob(w http.ResponseWriter, r *http.Request, namespace string, blobId string) error {
	f, err := ioutil.TempFile(b.tempDir, "blob")
	if err != nil {
		return chained.Error(err, "error creating temp file")
	}
	defer os.Remove(f.Name())
	_, err = io.Copy(f, r.Body)
	if err != nil {
		return chained.Error(err, "error copying posted body to temp file")
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return chained.Error(err, "error seeking temp file")
	}
	err = b.blobStore.PutBlob(namespace, blobId, f)
	if err != nil {
		return chained.Error(err, "error writing blob")
	}
	return nil
}

func (b *BlobServer) ListenAndServe() error {
	endpoint := ":8080"
	http.HandleFunc("/blob/", b.blobHandler)
	glog.Info("Blobserver listening on ", endpoint)
	err := http.ListenAndServe(endpoint, nil)
	if err != nil {
		return chained.Error(err, "Error listening for blobserver")
	}
	return nil
}
