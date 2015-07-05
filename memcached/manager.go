package memcached

import (
	"github.com/kopeio/kope/process"
	"strconv"
	"time"
	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/base"
)

const DefaultMemory = 128

type Manager struct {
	base.KopeBaseManager
	process  *process.Process
}

func (m *Manager) Configure() error {
	err := m.ConfigureMemory()
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
			glog.Info("Setting memcached memory to ", memoryLimitMB)
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
	argv := []string{"/usr/bin/memcached"}
	argv = append(argv, "-p", "11211")
	argv = append(argv, "-u", "memcache")
	argv = append(argv, "-l", "0.0.0.0")
	argv = append(argv, "-m", strconv.Itoa(m.MemoryMB))

	config := &process.ProcessConfig{}
	config.Argv = argv

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
