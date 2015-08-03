package l7proxy

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/chained"
)

type KubernetesBackendProvider struct {
	registry kubernetesRegistry
}

var _ BackendProvider = &KubernetesBackendProvider{}

func NewKubernetesBackendProvider() (*KubernetesBackendProvider, error) {
	k := &KubernetesBackendProvider{}
	err := k.registry.init()
	if err != nil {
		return nil, chained.Error(err, "error initializing kubernetes registry")
	}
	return k, nil
}

func (b *KubernetesBackendProvider) PickBackend(r *http.Request, host string, backendCookie string, skip BackendIdList) *Backend {
	if host == "" {
		// TODO: Default site?
		return nil
	}

	return b.registry.PickBackend(host, backendCookie, skip)
}

func (r *kubernetesRegistry) PickBackend(host string, backendCookie string, skip BackendIdList) *Backend {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.findServiceByHost(host, locked)
	if service == nil {
		glog.V(2).Infof("No service registered for host %q", host)
		return nil
	}

	// First try to match the backend cookie
	if backendCookie != "" && !skip.Contains(backendCookie) {
		backend, found := service.BackendsById[backendCookie]
		if found {
			return backend
		}
	}

	backendCount := len(service.Backends)

	// Randomly pick a backend (that is not in skip)
	startPos := rand.Intn(backendCount)
	pos := startPos
	for {
		sb := &service.Backends[pos]
		if !skip.Contains(sb.Id) {
			return sb
		}

		pos++

		if pos >= backendCount {
			pos = 0
		}

		if pos == startPos {
			break
		}

	}

	return nil
}

type serviceData struct {
	Host         string
	Backends     []Backend
	BackendsById map[string]*Backend
}

type backendDataStore struct {
	services       map[string]*serviceData
	servicesByHost map[string]*serviceData

	mutex sync.Mutex
}

func (d *backendDataStore) init() {
	d.services = make(map[string]*serviceData)
	d.servicesByHost = make(map[string]*serviceData)
}

type holdsLock struct {
}

func (d *backendDataStore) lock() holdsLock {
	d.mutex.Lock()
	var l holdsLock
	return l
}

func (d *backendDataStore) unlock(_ holdsLock) {
	d.mutex.Unlock()
}

func (d *backendDataStore) getService(namespace, name string, create bool, _ holdsLock) *serviceData {
	key := namespace + "_" + name

	service := d.services[key]
	if service == nil && create {
		service = &serviceData{}
		d.services[key] = service
	}
	return service
}

func (d *backendDataStore) deleteService(namespace, name string, _ holdsLock) {
	key := namespace + "_" + name

	delete(d.services, key)
}

type kubernetesRegistry struct {
	//	mutex sync.Mutex

	k8s  *kope.Kubernetes
	data backendDataStore
}

func sliceBackendsEqual(l, r []Backend) bool {
	if len(l) != len(r) {
		return false
	}
	for i := range l {
		if l[i] != r[i] {
			return false
		}
	}
	return true
}

func (s *serviceData) updateEndpoints(e *api.Endpoints, _ holdsLock) bool {
	var backends []Backend
	if e != nil {
		for i := range e.Subsets {
			subset := &e.Subsets[i]

			// Look for an http port: if there is one service use that,
			// otherwise look for a service named "http" or "http-server"
			httpPort := 0
			if len(subset.Ports) == 1 {
				httpPort = subset.Ports[0].Port
			} else {
				for j := range subset.Ports {
					port := &subset.Ports[j]
					if port.Name == "http" {
						httpPort = port.Port
					} else if port.Name == "http-server" && httpPort == 0 {
						httpPort = port.Port
					}
				}
			}

			if httpPort != 0 {
				for j := range subset.Addresses {
					address := &subset.Addresses[j]

					targetId := ""
					targetRef := address.TargetRef
					if targetRef != nil {
						targetId = string(targetRef.UID)
					}

					backend := Backend{}
					backend.Id = targetId
					backend.Endpoint = address.IP + ":" + strconv.Itoa(httpPort)
					backends = append(backends, backend)
				}
			}
		}
	}

	dirty := false
	if !sliceBackendsEqual(backends, s.Backends) {
		dirty = true
		s.Backends = backends
		backendMap := make(map[string]*Backend)
		for i := range s.Backends {
			backend := &s.Backends[i]
			if backend.Id == "" {
				glog.Warning("Ignoring backend with empty id: ", backend.Endpoint)
				continue
			}
			backendMap[backend.Id] = backend
		}
		s.BackendsById = backendMap
		glog.V(2).Infof("Updated backends for %q: %v", e.Name, backends)
	}
	return dirty
}

func (d *serviceData) updateService(s *api.Service, _ holdsLock) bool {
	var host string
	if s != nil {
		for k, v := range s.Labels {
			if k == "http.host" {
				host = v
			}
		}
	}
	dirty := false
	if d.Host != host {
		glog.V(2).Infof("Updating service %q host to %q", s.Name, host)
		d.Host = host
		dirty = true
	}
	return dirty
}

// Finds the service with the specified host.  Only valid while lock is held.
func (s *backendDataStore) findServiceByHost(host string, _ holdsLock) *serviceData {
	service, found := s.servicesByHost[host]
	if !found {
		return nil
	}
	return service
}

func (r *kubernetesRegistry) AddEndpoints(e *api.Endpoints) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.getService(e.Namespace, e.Name, true, locked)
	service.updateEndpoints(e, locked)

	if glog.V(2) {
		glog.Infof("Add endpoint %s::%s : %v", e.Namespace, e.Name, e)
	}
}

func (r *kubernetesRegistry) DeleteEndpoints(e *api.Endpoints) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.getService(e.Namespace, e.Name, false, locked)
	if service != nil {
		service.updateEndpoints(nil, locked)
	}

	if glog.V(2) {
		glog.Infof("Delete endpoint %s::%s : %v", e.Namespace, e.Name, e)
	}
}

func (r *kubernetesRegistry) UpdateEndpoints(oldEndpoints, e *api.Endpoints) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.getService(e.Namespace, e.Name, true, locked)
	service.updateEndpoints(e, locked)

	if glog.V(2) {
		glog.Infof("Update endpoint %s::%s : %v", e.Namespace, e.Name, e)
	}
}

func (r *kubernetesRegistry) AddService(e *api.Service) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.getService(e.Namespace, e.Name, true, locked)
	oldHost := service.Host
	service.updateService(e, locked)
	newHost := service.Host

	if oldHost != newHost {
		if oldHost != "" {
			delete(r.data.servicesByHost, oldHost)
		}
		if newHost != "" {
			r.data.servicesByHost[newHost] = service
		}
	}

	if glog.V(2) {
		glog.Infof("Add service %s::%s : %v", e.Namespace, e.Name, e)
	}
}

func (r *kubernetesRegistry) DeleteService(e *api.Service) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	r.data.deleteService(e.Namespace, e.Name, locked)

	if glog.V(2) {
		glog.Infof("Delete service %s::%s : %v", e.Namespace, e.Name, e)
	}
}

func (r *kubernetesRegistry) UpdateService(old, s *api.Service) {
	locked := r.data.lock()
	defer r.data.unlock(locked)

	service := r.data.getService(s.Namespace, s.Name, true, locked)
	service.updateService(s, locked)

	if glog.V(2) {
		glog.Infof("Update service %s::%s : %v", s.Namespace, s.Name, s)
	}
}

func (r *kubernetesRegistry) init() error {
	r.data.init()

	k8s, err := kope.NewKubernetesClient()
	if err != nil {
		return fmt.Errorf("error connecting to kubernetes: %v", err)
	}
	r.k8s = k8s

	// TODO: Error handling here
	// TODO: We don't really _need_ services; we could look for a tag on an RC
	go r.k8s.WatchEndpoints(r)
	go r.k8s.WatchServices(r)

	return nil
}
