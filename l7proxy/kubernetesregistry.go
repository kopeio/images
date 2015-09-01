package l7proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/chained"
	"k8s.io/kubernetes/pkg/api"
)

type CertificateProvider interface {
	GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error)
}

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

func (b *KubernetesBackendProvider) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cn := clientHello.ServerName
	cn = strings.ToLower(cn)
	secret := b.registry.data.findSecretByCN(cn)
	if secret == nil {
		// Check for wildcard
		dotIndex := strings.Index(cn, ".")
		if dotIndex != -1 {
			cn = "*" + cn[dotIndex:]
			secret = b.registry.data.findSecretByCN(cn)
		}
	}
	if secret == nil {
		return nil, nil
	}

	return secret.GetCertificate()
}

func (r *kubernetesRegistry) PickBackend(host string, backendCookie string, skip BackendIdList) *Backend {
	service := r.data.findServiceByHost(host)
	if service == nil {
		glog.V(2).Infof("No service registered for host %q", host)
		return nil
	}

	backendCount := len(service.Backends)

	if backendCount == 0 {
		return nil
	}

	// First try to match the backend cookie
	if backendCookie != "" && !skip.Contains(backendCookie) {
		backend, found := service.BackendsById[backendCookie]
		if found {
			return backend
		}
	}

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

type secretData struct {
	CN      string
	CertRaw []byte
	KeyRaw  []byte

	mutex       sync.Mutex
	certificate *tls.Certificate
	parseError  error
}

type backendDataStore struct {
	services       map[string]*serviceData
	servicesByHost map[string]*serviceData

	secrets     map[string]*secretData
	secretsByCN map[string]*secretData

	mutex sync.Mutex
}

func (d *backendDataStore) init() {
	d.services = make(map[string]*serviceData)
	d.servicesByHost = make(map[string]*serviceData)
	d.secrets = make(map[string]*secretData)
	d.secretsByCN = make(map[string]*secretData)
}

func (d *backendDataStore) updateEndpoints(namespace, name string, endpoints *api.Endpoints) {
	backends := buildBackends(endpoints)

	key := namespace + "_" + name

	d.mutex.Lock()
	defer d.mutex.Unlock()

	oldData := d.services[key]
	newData := &serviceData{}
	if oldData != nil {
		*newData = *oldData
	}

	changed := false
	if endpoints == nil {
		if newData.Backends != nil {
			glog.V(2).Infof("Delete endpoints %s::%s", namespace, name)

			newData.Backends = nil
			changed = true
		}
	} else {
		if !sliceBackendsEqual(backends, newData.Backends) {
			glog.V(2).Infof("Update endpoints %s::%s : %v", namespace, name, backends)

			newData.Backends = backends
			backendMap := make(map[string]*Backend)
			for i := range backends {
				backend := &backends[i]
				if backend.Id == "" {
					glog.Warning("Ignoring backend with empty id: ", backend.Endpoint)
					continue
				}
				backendMap[backend.Id] = backend
			}
			newData.BackendsById = backendMap

			changed = true
		}
	}

	if changed {
		d.updateData(key, oldData, newData)
	}
}

func (d *backendDataStore) updateService(namespace, name string, service *api.Service) {
	key := namespace + "_" + name

	d.mutex.Lock()
	defer d.mutex.Unlock()

	oldData, _ := d.services[key]
	newData := &serviceData{}
	if oldData != nil {
		*newData = *oldData
	}

	changed := false
	if service == nil {
		if oldData != nil {
			glog.V(2).Infof("Delete service %s::%s", namespace, name)
			newData = nil
			changed = true
		}
	} else {
		host := findHost(service)
		if newData.Host != host {
			glog.V(2).Infof("Update service %s::%s : host=%s", namespace, name, host)
			newData.Host = host
			changed = true
		}
	}

	if changed {
		d.updateData(key, oldData, newData)
	}
}

func (d *backendDataStore) updateSecret(namespace, name string, s *api.Secret) {
	key := namespace + "_" + name

	certRaw := findSecretValue(s, ".crt")
	keyRaw := findSecretValue(s, ".key")
	cn := findSecretCN(s)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	oldData, _ := d.secrets[key]
	newData := &secretData{}
	if oldData != nil {
		*newData = *oldData
	}

	changed := false
	if s == nil {
		if oldData != nil {
			glog.V(2).Infof("Delete secret %s::%s", namespace, name)
			newData = nil
			changed = true
		}
	} else {
		if !bytes.Equal(newData.CertRaw, certRaw) || !bytes.Equal(newData.KeyRaw, keyRaw) {
			glog.V(2).Infof("Update secret %s::%s", namespace, name)
			newData.CertRaw = certRaw
			newData.KeyRaw = keyRaw
			newData.certificate = nil
			newData.parseError = nil
			changed = true
		}

		if newData.CN != cn {
			glog.V(2).Infof("Update secret %s::%s CN=%s", namespace, name, cn)
			newData.CN = cn
			changed = true
		}
	}

	if changed {
		d.updateSecretData(key, oldData, newData)
	}
}

// Updates the data structures.  Must hold lock.
func (d *backendDataStore) updateData(key string, oldData, newData *serviceData) {
	if newData != nil {
		d.services[key] = newData
		if oldData != nil && oldData.Host != newData.Host {
			delete(d.servicesByHost, oldData.Host)
		}
		d.servicesByHost[newData.Host] = newData
	} else {
		delete(d.services, key)
		if oldData != nil && oldData.Host != "" {
			delete(d.servicesByHost, oldData.Host)
		}
	}
}

// Updates the data structures.  Must hold lock.
func (d *backendDataStore) updateSecretData(key string, oldData, newData *secretData) {
	if newData != nil {
		d.secrets[key] = newData
		if oldData != nil && oldData.CN != newData.CN {
			delete(d.secretsByCN, oldData.CN)
		}
		d.secretsByCN[newData.CN] = newData
	} else {
		delete(d.secrets, key)
		if oldData != nil && oldData.CN != "" {
			delete(d.secretsByCN, oldData.CN)
		}
	}
}

func (s *backendDataStore) findServiceByHost(host string) *serviceData {
	s.mutex.Lock()
	service, _ := s.servicesByHost[host]
	s.mutex.Unlock()
	return service
}

func (s *backendDataStore) findSecretByCN(cn string) *secretData {
	s.mutex.Lock()
	secret, _ := s.secretsByCN[cn]
	s.mutex.Unlock()
	return secret
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

func buildBackends(e *api.Endpoints) []Backend {
	if e == nil {
		return nil
	}
	var backends []Backend
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
	return backends
}

func findHost(s *api.Service) string {
	var host string
	if s != nil {
		for k, v := range s.Labels {
			if k == "http.host" {
				host = v
			}
		}
		for k, v := range s.Annotations {
			if k == "http.host" {
				host = v
			}
		}
	}
	return host
}

func findSecretCN(s *api.Secret) string {
	var cn string
	if s != nil {
		for k, v := range s.Annotations {
			if k == "cert-cn" {
				cn = v
			}
		}
	}
	return cn
}

func findSecretValue(s *api.Secret, suffix string) []byte {
	var found []byte
	if s != nil {
		for k, v := range s.Data {
			if strings.HasSuffix(k, suffix) {
				found = v
				break
			}
		}
	}
	return found
}

func (r *kubernetesRegistry) AddEndpoints(e *api.Endpoints) {
	r.data.updateEndpoints(e.Namespace, e.Name, e)
}

func (r *kubernetesRegistry) DeleteEndpoints(e *api.Endpoints) {
	r.data.updateEndpoints(e.Namespace, e.Name, nil)
}

func (r *kubernetesRegistry) UpdateEndpoints(oldEndpoints, e *api.Endpoints) {
	r.data.updateEndpoints(e.Namespace, e.Name, e)
}

func (r *kubernetesRegistry) AddService(s *api.Service) {
	r.data.updateService(s.Namespace, s.Name, s)
}

func (r *kubernetesRegistry) DeleteService(s *api.Service) {
	r.data.updateService(s.Namespace, s.Name, nil)
}

func (r *kubernetesRegistry) UpdateService(old, s *api.Service) {
	r.data.updateService(s.Namespace, s.Name, s)
}

func (r *kubernetesRegistry) AddSecret(s *api.Secret) {
	r.data.updateSecret(s.Namespace, s.Name, s)
}

func (r *kubernetesRegistry) DeleteSecret(s *api.Secret) {
	r.data.updateSecret(s.Namespace, s.Name, nil)
}

func (r *kubernetesRegistry) UpdateSecret(old, s *api.Secret) {
	r.data.updateSecret(s.Namespace, s.Name, s)
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
	go r.k8s.WatchSecrets(r)

	return nil
}

func (s *secretData) GetCertificate() (*tls.Certificate, error) {
	// TODO: Just use mutex to avoid repeated parsing??
	if s.CertRaw == nil || s.KeyRaw == nil {
		return nil, nil
	}

	s.mutex.Lock()
	if s.certificate != nil {
		s.mutex.Unlock()
		return s.certificate, nil
	}
	defer s.mutex.Unlock()

	if s.parseError != nil {
		return nil, s.parseError
	}

	parsed, err := tls.X509KeyPair(s.CertRaw, s.KeyRaw)
	if err != nil {
		s.parseError = err
		glog.V(2).Info("Error parsing certificate for %s: %v", s.CN, err)
		return nil, err
	} else {
		s.certificate = &parsed
		return s.certificate, nil
	}
}
