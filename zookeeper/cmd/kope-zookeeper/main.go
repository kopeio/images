package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/kopeio/kope/zookeeper"
	"math/rand"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	manager := &zookeeper.Manager{}
	err := manager.Manage()
	if err != nil {
		glog.Fatalf("manager exited with error: %v", err)
	}
}
