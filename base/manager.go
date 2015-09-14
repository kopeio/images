package base

import (
	"fmt"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/chained"
)

var selfPodMissingTombstone = &kope.KopePod{}

type KopeBaseManager struct {
	MemoryMB  int
	ClusterID string

	NodeID           *string
	KubernetesClient *kope.Kubernetes

	// Cached self-pod (access through GetSelfPod, as this may be selfPodMissingTombstone)
	selfPod *kope.KopePod
}

func (m *KopeBaseManager) Configure() error {
	selfPod, err := m.GetSelfPod()
	if err != nil {
		return err
	}

	memory := os.Getenv("MEMORY_LIMIT")
	if memory == "" {
		if selfPod != nil {
			if len(selfPod.Pod.Spec.Containers) > 1 {
				glog.Warning("Found multiple containers in pod, choosing arbitrarily")
			}
			memoryLimit := selfPod.Pod.Spec.Containers[0].Resources.Limits.Memory()
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

	clusterID := ""
	if selfPod != nil {
		labels := selfPod.Pod.Labels
		clusterID = labels["kope.io/clusterid"]
	}
	if clusterID != "" {
		glog.Info("Found clusterid: ", clusterID)
		m.ClusterID = clusterID
	}
	return nil
}

func (m *KopeBaseManager) GetClusterMap() (map[string]*kope.KopePod, error) {
	if m.ClusterID == "" {
		return nil, nil
	}

	selfPod, err := m.GetSelfPod()
	if err != nil {
		return nil, err
	}

	if selfPod == nil {
		return nil, nil
	}

	clusterID := m.ClusterID

	clusterMap, err := selfPod.GetClusterMap(clusterID)
	if err != nil {
		return nil, err
	}

	glog.Info("Cluster-map:")
	for k, v := range clusterMap {
		name := ""
		if v != nil {
			name = v.Pod.Name
		}
		glog.Info("\t", k, "\t", name)
	}

	return clusterMap, nil
}

func (m *KopeBaseManager) GetNodeId() (string, error) {
	nodeID := ""
	if m.NodeID != nil {
		nodeID = *m.NodeID
	}
	if nodeID == "" {
		selfPod, err := m.GetSelfPod()
		if err != nil {
			return "", err
		}

		if selfPod != nil {
			labels := selfPod.Pod.Labels
			nodeID = labels["kope.io/clusterid"]
		}

		if nodeID == "" {
			volumes, err := selfPod.GetVolumes()
			if err != nil {
				return "", err
			}

			if nodeID == "" {
				for _, volume := range volumes {
					pvc, err := volume.GetPersistentVolumeClaim()
					if err != nil {
						return "", err
					}
					if pvc == nil {
						continue
					}
					labels := pvc.Labels
					nodeID = labels["kope.io/clusterid"]
					if nodeID != "" {
						break
					}
				}
			}

			if nodeID == "" {
				for _, volume := range volumes {
					pv, err := volume.GetPersistentVolume()
					if err != nil {
						return "", err
					}
					if pv == nil {
						continue
					}
					labels := pv.Labels
					nodeID = labels["kope.io/clusterid"]
					if nodeID != "" {
						break
					}
				}
			}

		}
	}

	if nodeID != "" {
		glog.Info("Found nodeid: ", nodeID)
	}

	m.NodeID = &nodeID

	return nodeID, nil
}

func (m *KopeBaseManager) GetSelfPod() (*kope.KopePod, error) {
	selfPod := m.selfPod
	if selfPod != nil {
		if selfPod == selfPodMissingTombstone {
			return nil, nil
		} else {
			return selfPod, nil
		}
	}

	if m.KubernetesClient != nil {
		k8sPod, err := m.KubernetesClient.FindSelfPod()
		if err != nil {
			return nil, chained.Error(err, "Unable to find self pod in kubernetes")
		}
		if k8sPod != nil {
			pod := &kope.KopePod{}
			pod.Pod = k8sPod
			pod.KubernetesClient = m.KubernetesClient
			m.selfPod = pod
			selfPod = pod
		} else {
			m.selfPod = selfPodMissingTombstone
		}
	}

	return selfPod, nil
}

func (m *KopeBaseManager) GetLabels() (map[string]string, error) {
	selfPod, err := m.GetSelfPod()
	if m != nil {
		return nil, err
	}

	if selfPod != nil {
		return selfPod.Pod.Labels, nil
	}
	return nil, nil
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
