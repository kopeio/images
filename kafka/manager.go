package kafka

import (
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"os"
	"time"
)

const DefaultMemory = 256

type Manager struct {
	base.KopeBaseManager
	process *process.Process
}

type Config struct {
	AdvertisedHostName string
	ZookeeperConnect   string
	BrokerID           int
}

func (m *Manager) Configure() error {
	err := m.KopeBaseManager.Configure()
	if err != nil {
		return err
	}

	if m.MemoryMB == 0 {
		m.MemoryMB = DefaultMemory
	} else {
		memoryLimitMB := m.MemoryMB

		// We leave 32 MB for overhead (connections etc)
		memoryLimitMB -= 32

		if memoryLimitMB < 0 {
			glog.Warning("Memory limit was too low; ignoring")
			m.MemoryMB = DefaultMemory
		} else {
			glog.Info("Setting memory to ", memoryLimitMB)
			m.MemoryMB = memoryLimitMB
		}
	}

	return nil
}

func (m *Manager) Manage() error {
	err := m.Init()
	if err != nil {
		return chained.Error(err, "error initializing")
	}

	err = m.Configure()
	if err != nil {
		return chained.Error(err, "error configuring")
	}

	process, err := m.Start()
	if err != nil {
		return chained.Error(err, "error starting")
	}
	m.process = process

	for {
		time.Sleep(5 * time.Second)
	}

	return nil
}

func (m *Manager) Start() (*process.Process, error) {
	for _, dir := range []string{"/data/conf", "/data/logs"} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return nil, chained.Error(err, "error doing mkdir on: ", dir)
		}
	}

	clusterMap, err := m.GetClusterMap()
	if err != nil {
		return nil, err
	}

	podIP, err := kope.FindSelfPodIP()
	if err != nil {
		return nil, err
	}

	var config Config
	config.BrokerID = 1
	config.ZookeeperConnect = "zookeeper:2181"
	config.AdvertisedHostName = podIP.String()

	if len(clusterMap) != 0 && len(clusterMap) != 1 {
		glog.Fatal("Detected cluster configuration but not implemented")
	}

	err = kope.WriteTemplate("/data/conf/server.properties", &config)
	if err != nil {
		return nil, err
	}

	// TODO: Actually set memory

	argv := []string{"/opt/kafka/bin/kafka-server-start.sh"}
	argv = append(argv, "/data/conf/server.properties")

	processConfig := &process.ProcessConfig{}
	processConfig.Argv = argv

	process, err := processConfig.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
