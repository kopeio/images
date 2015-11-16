package cassandra

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
	config  Config
}

type Config struct {
	CommitLogDir string
	DataDir      string
}

func (m *Manager) Configure() error {
	err := m.KopeBaseManager.Configure()
	if err != nil {
		return err
	}

	m.config.CommitLogDir = "/data/cassandra/logs"
	m.config.DataDir = "/data/cassandra/data"

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
	for _, dir := range []string{"/data/conf", m.config.CommitLogDir, m.config.DataDir} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return nil, chained.Error(err, "error doing mkdir on: ", dir)
		}
	}

	clusterMap, err := m.GetClusterMap()
	if err != nil {
		return nil, err
	}

	if len(clusterMap) != 0 {
		glog.Info("Detected cluster configuration")
		glog.Fatalf("Cluster not yet implemented")
	}

	err = kope.WriteTemplate("/data/conf/cassandra.yaml", &m.config)
	if err != nil {
		return nil, err
	}

	// TODO: Actually set memory

	argv := []string{"/opt/cassandra/bin/cassandra"}
	argv = append(argv, "-f")

	env := []string{}
	env = append(env, "CASSANDRA_CONF=" + "/data/conf")

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.Env = env

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
