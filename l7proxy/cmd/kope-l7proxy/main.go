package main

import (
	"crypto/tls"
	"flag"
	"math/rand"
	"time"

	"github.com/golang/glog"
	"github.com/kopeio/kope/l7proxy"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	//backendProvider := l7proxy.NewDummyBackendProvider()
	backendProvider, err := l7proxy.NewKubernetesBackendProvider()
	if err != nil {
		glog.Fatalf("error initializing kubernetes backend provider: %v", err)
	}

	handler := l7proxy.NewProxyingHandler(backendProvider)

	tlsConfig := &tls.Config{}
	tlsConfig.GetCertificate = backendProvider.GetCertificate

	httpListener := l7proxy.NewHTTPListener(":80", handler)
	httpsListener := l7proxy.NewHTTPSListener(":443", handler, tlsConfig)

	proxy := l7proxy.NewProxyServer()
	proxy.AddListener(httpListener)
	proxy.AddListener(httpsListener)

	err = proxy.ListenAndServe()
	if err != nil {
		glog.Fatalf("proxy exited with error: %v", err)
	}
}
