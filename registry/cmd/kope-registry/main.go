package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/registry"
	"math/rand"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	manager := &registry.Manager{}
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
