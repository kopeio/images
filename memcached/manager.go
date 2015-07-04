package memcached

import (
	"fmt"
	"github.com/kopeio/kope/process"
	"os"
	"strconv"
	"time"
)

const DefaultMemory = 128

type MemcacheManager struct {
	MemoryMb int
	process  *process.Process
}

func (m *MemcacheManager) Configure() error {
	memory := os.Getenv("MEMCACHE_MEMORY")
	if memory == "" {
		m.MemoryMb = DefaultMemory
	} else {
		var err error
		m.MemoryMb, err = strconv.Atoi(memory)
		if err != nil {
			return fmt.Errorf("error parsing MEMCACHE_MEMORY: %v", memory)
		}
	}
	return nil
}

func (m *MemcacheManager) Manage() error {
	err := m.Configure()
	if err != nil {
		return fmt.Errorf("error configuring memcached: %v", err)
	}

	process, err := m.Start()
	if err != nil {
		return fmt.Errorf("error starting memcached: %v", err)
	}
	m.process = process

	for {
		time.Sleep(5 * time.Second)
	}

	return nil
}

func (m *MemcacheManager) Start() (*process.Process, error) {
	argv := []string{"/usr/bin/memcached"}
	argv = append(argv, "-p", "11211")
	argv = append(argv, "-u", "memcache")
	argv = append(argv, "-l", "0.0.0.0")
	argv = append(argv, "-m", strconv.Itoa(m.MemoryMb))

	config := &process.ProcessConfig{}
	config.Argv = argv

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
