package main

import (
	"flag"
	"github.com/golang/glog"
	"math/rand"
	"time"
	"github.com/kopeio/kope/postgres"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	manager := &postgres.Manager{}
	err := manager.Manage()
	if err != nil {
		glog.Fatalf("manager exited with error: %v", err)
	}
}
