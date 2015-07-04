package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/kopeio/kope/memcached"
	"math/rand"
	"time"
	"github.com/kopeio/kope"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	manager := &memcached.MemcacheManager{}
	if kope.IsKubernetes() {
		glog.Infof("Detected kubernetes")
		client, err := kope.NewKubernetesClient()
		if err != nil {
			glog.Fatalf("error building kubernetes client: %v", err)
		}
		manager.KubernetesClient = client
	}

	err := manager.Manage()
	if err != nil {
		glog.Fatalf("manager exited with error: %v", err)
	}
}
