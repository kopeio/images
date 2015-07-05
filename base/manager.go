package base

import (
	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/chained"
	"os"
	"strconv"
)

type KopeBaseManager struct {
	MemoryMB         int
	KubernetesClient *kope.Kubernetes
}

func (m *KopeBaseManager) ConfigureMemory() error {
	var selfPod *api.Pod
	if m.KubernetesClient != nil {
		var err error
		selfPod, err = m.KubernetesClient.FindSelfPod()
		if err != nil {
			return chained.Error(err, "Unable to find self pod in kubernetes")
		}
	}

	memory := os.Getenv("MEMORY_LIMIT")
	if memory == "" {
		if selfPod != nil {
			if len(selfPod.Spec.Containers) > 1 {
				glog.Warning("Found multiple containers in pod, choosing arbitrarily")
			}
			memoryLimit := selfPod.Spec.Containers[0].Resources.Limits.Memory()
			if memoryLimit != nil {
				memoryLimitBytes := memoryLimit.Value()
				if memoryLimitBytes > 0 {
					memoryLimitMB := int(memoryLimitBytes / (1024 * 1024))
					glog.Info("Found container memory limit: ", memoryLimitMB)

					m.MemoryMB = memoryLimitMB
				}
			}
		}
	} else {
		memoryMB, err := strconv.Atoi(memory)
		if err != nil {
			return fmt.Errorf("error parsing MEMORY_LIMIT: %v", memory)
		}
		m.MemoryMB = memoryMB
	}
	return nil
}

func (m *KopeBaseManager) Init() error {
	if kope.IsKubernetes() {
		glog.Infof("Detected kubernetes")
		client, err := kope.NewKubernetesClient()
		if err != nil {
			return chained.Error(err, "error building kubernetes client")
		}
		m.KubernetesClient = client
	}
	return nil
}
