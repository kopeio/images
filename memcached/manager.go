package memcached

import (
	"fmt"
	"github.com/kopeio/kope/process"
	"os"
	"strconv"
	"time"
	"github.com/kopeio/kope"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
)

const DefaultMemory = 128

type MemcacheManager struct {
	MemoryMb int
	process  *process.Process
	KubernetesClient *kope.Kubernetes
}

func (m *MemcacheManager) Configure() error {
	var selfPod *api.Pod
	if m.KubernetesClient != nil {
		var err error
		selfPod, err = m.KubernetesClient.FindSelfPod()
		if err != nil {
			return chained.Error(err, "Unable to find self pod in kubernetes")
		}
	}

	memory := os.Getenv("MEMCACHE_MEMORY")
	if memory == "" {
		m.MemoryMb = DefaultMemory

		if selfPod != nil {
			if len(selfPod.Spec.Containers) > 0 {
				glog.Warning("Found multiple containers in pod, choosing arbitrarily")
			}
			memoryLimit := selfPod.Spec.Containers[0].Resources.Limits.Memory()
			if memoryLimit != nil {
				memoryLimitBytes := memoryLimit.Value()
				if memoryLimitBytes > 0 {
					memoryLimitMB := int(memoryLimitBytes / (1024 * 1024))

					// We leave 32 MB for overhead (connections etc)
					memoryLimitMB -= 32

					if memoryLimitMB < 0 {
						glog.Warning("Memory limit was too low; ignoring")
					} else {
						m.MemoryMb = memoryLimitMB
					}
				}
			}
		}
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
