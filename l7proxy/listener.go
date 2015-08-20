package l7proxy

import (
	crypto_rand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
)

func NewHTTPListener(endpoint string, handler http.Handler) *Listener {
	l := &Listener{}
	l.endpoint = endpoint
	l.handler = handler
	return l
}

func NewHTTPSListener(endpoint string, handler http.Handler, tlsConfig *tls.Config) *Listener {
	l := &Listener{}
	l.endpoint = endpoint
	l.handler = handler
	l.tlsConfig = tlsConfig
	return l
}

type Listener struct {
	endpoint  string
	tlsConfig *tls.Config
	handler   http.Handler
}

func (l *Listener) listenAndServe() error {
	s := &http.Server{
		Addr:           l.endpoint,
		Handler:        l.handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	// TODO: ConnState ?
	// TODO: ErrorLog ?

	if l.tlsConfig != nil {
		s.TLSConfig = l.tlsConfig
		// TODO: TLSNextProto

		// We want to use ListenAndServeTLS, but it _requires_ a valid certFile and keyFile
		// Create dummy values
		// TODO: We could have this be the 'default' cert instead
		certBytes, keyBytes, err := generateSelfSignedCert()
		if err != nil {
			return chained.Error(err, "error generating self-signed cert")
		}
		certFile, err := writeTempFile(certBytes)
		if err != nil {
			return chained.Error(err, "error writing temp cert file")
		}
		defer loggedRemoveFile(certFile)
		keyFile, err := writeTempFile(keyBytes)
		if err != nil {
			return chained.Error(err, "error writing temp key file")
		}
		defer loggedRemoveFile(keyFile)

		glog.Info("HTTPS listening on: ", l.endpoint)
		err = s.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			return chained.Error(err, "error starting https listener")
		}
	} else {
		glog.Info("HTTP listening on: ", l.endpoint)
		err := s.ListenAndServe()
		if err != nil {
			return chained.Error(err, "error starting http listener")
		}
	}
	return nil
}

func loggedRemoveFile(path string) {
	err := os.Remove(path)
	if err != nil {
		glog.Warning("error removing file ", path, ": ", err)
	}
}

func writeTempFile(contents []byte) (string, error) {
	f, err := ioutil.TempFile("", "tmp")
	if err != nil {
		return "", chained.Error(err, "error creating temp file")
	}

	defer func() {
		if f != nil {
			err := f.Close()
			if err != nil {
				glog.Warning("error closing temp file: ", err)
			}
			loggedRemoveFile(f.Name())
		}
	}()

	_, err = f.Write(contents)
	if err != nil {
		return "", chained.Error(err, "error writing temp file")
	}
	err = f.Close()
	if err != nil {
		return "", chained.Error(err, "error closing temp file")
	}
	path := f.Name()
	f = nil
	return path, nil
}

// Based on (BSD) http://golang.org/src/crypto/tls/generate_cert.go
func generateSelfSignedCert() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(crypto_rand.Reader, 2048)
	if err != nil {
		return nil, nil, chained.Error(err, "error generating rsa key")
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	serialNumber := big.NewInt(1)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Dummy certificate"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.DNSNames = append(template.DNSNames, "localhost")

	derBytes, err := x509.CreateCertificate(crypto_rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, chained.Error(err, "failed to create certificate")
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certBytes, keyBytes, nil
}
