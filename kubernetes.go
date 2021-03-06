package kope

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller/framework"
	kcontrollerFramework "k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
)

const DefaultKubecfgFile = "/etc/kubernetes/kubeconfig"

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 60 * time.Second
)

type EndpointWatch interface {
	AddEndpoints(e *api.Endpoints)
	DeleteEndpoints(e *api.Endpoints)
	UpdateEndpoints(oldEndpoints, newEndpoints *api.Endpoints)
}

type ServiceWatch interface {
	AddService(s *api.Service)
	DeleteService(s *api.Service)
	UpdateService(oldService, newService *api.Service)
}

type SecretWatch interface {
	AddSecret(s *api.Secret)
	DeleteSecret(s *api.Secret)
	UpdateSecret(oldSecret, newSecret *api.Secret)
}

type Kubernetes struct {
	kubeClient *kclient.Client
}

func IsKubernetes() bool {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	return host != ""
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	glog.Warning("Got unexpected error checking if file (%s)  exists: %v", path, err)
	return false
}

func getKubeMasterUrl() (string, error) {
	portString := os.Getenv("KUBERNETES_SERVICE_PORT")
	if portString == "" {
		portString = "443"
	}
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	var s string
	if portString == "80" {
		s = "http://" + host
	} else if portString == "443" {
		s = "https://" + host
	} else {
		s = "https://" + host + ":" + portString
	}
	parsedUrl, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("failed to determine kubernetes url; got: %s", s)
	}
	if parsedUrl.Scheme == "" || parsedUrl.Host == "" || parsedUrl.Host == ":" {
		return "", fmt.Errorf("invalid kubernetes url: %s", s)
	}
	masterUrl := parsedUrl.String()
	glog.V(2).Info("Using kubernetes master url: ", masterUrl)
	return masterUrl, nil
}

func getKubeConfig(masterUrl string) (*kclient.Config, error) {
	s := "${KUBECFG_FILE}"
	p := os.ExpandEnv(s)

	if p == "" {
		if FileExists(DefaultKubecfgFile) {
			p = DefaultKubecfgFile
		}
	}

	if FileExists(p) {
		glog.Info("Using kubecfg file: ", p)

		overrides := &kclientcmd.ConfigOverrides{}
		if masterUrl != "" {
			overrides.ClusterInfo.Server = masterUrl
		}

		config, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&kclientcmd.ClientConfigLoadingRules{ExplicitPath: p},
			overrides).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("error loading kubecfg file (%s): %v", p, err)
		}
		return config, nil
	} else {
		glog.Warning("No kubecfg file found; using default in-cluster configuration")

		return kclient.InClusterConfig()
	}
}

// TODO: evaluate using pkg/client/clientcmd
func newKubeClient() (*kclient.Client, error) {
	masterUrl, err := getKubeMasterUrl()
	if err != nil {
		return nil, err
	}

	config, err := getKubeConfig(masterUrl)
	if err != nil {
		return nil, err
	}
	glog.Infof("Using %s for kubernetes master", config.Host)
	glog.Infof("Using kubernetes API %s", config.Version)
	return kclient.New(config)
}

func NewKubernetesClient() (*Kubernetes, error) {
	k := &Kubernetes{}
	kubeClient, err := newKubeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %v", err)
	}
	k.kubeClient = kubeClient
	return k, nil
}

func (k *Kubernetes) WatchEndpoints(watcher EndpointWatch) {
	glog.Info("Starting watch on k8s endpoints")

	// Watch all changes
	// TODO: filter
	lw := cache.NewListWatchFromClient(k.kubeClient, "endpoints", api.NamespaceAll, fields.Everything())

	var controller *kcontrollerFramework.Controller
	_, controller = framework.NewInformer(
		lw,
		&api.Endpoints{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				e, ok := o.(*api.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", o)
				} else {
					watcher.AddEndpoints(e)
				}
			},
			DeleteFunc: func(o interface{}) {
				e, ok := o.(*api.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", o)
				} else {
					watcher.DeleteEndpoints(e)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldE, ok := oldObj.(*api.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", oldObj)
				}
				newE, ok := newObj.(*api.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", newObj)
				}

				watcher.UpdateEndpoints(oldE, newE)
			},
		},
	)
	controller.Run(util.NeverStop)
}

func (k *Kubernetes) WatchServices(watcher ServiceWatch) {
	glog.Info("Starting watch on k8s services")

	// Watch all changes
	// TODO: filter
	lw := cache.NewListWatchFromClient(k.kubeClient, "services", api.NamespaceAll, fields.Everything())

	var controller *kcontrollerFramework.Controller
	_, controller = framework.NewInformer(
		lw,
		&api.Service{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				e, ok := o.(*api.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", o)
				} else {
					watcher.AddService(e)
				}
			},
			DeleteFunc: func(o interface{}) {
				e, ok := o.(*api.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", o)
				} else {
					watcher.DeleteService(e)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldE, ok := oldObj.(*api.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", oldObj)
				}
				newE, ok := newObj.(*api.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", newObj)
				}

				watcher.UpdateService(oldE, newE)
			},
		},
	)
	controller.Run(util.NeverStop)
}

func (k *Kubernetes) WatchSecrets(watcher SecretWatch) {
	glog.Info("Starting watch on k8s secrets")

	// Watch all changes
	// TODO: filter
	lw := cache.NewListWatchFromClient(k.kubeClient, "secrets", api.NamespaceAll, fields.Everything())

	var controller *kcontrollerFramework.Controller
	_, controller = framework.NewInformer(
		lw,
		&api.Secret{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				e, ok := o.(*api.Secret)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Secret", o)
				} else {
					watcher.AddSecret(e)
				}
			},
			DeleteFunc: func(o interface{}) {
				e, ok := o.(*api.Secret)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Secret", o)
				} else {
					watcher.DeleteSecret(e)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldE, ok := oldObj.(*api.Secret)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Secret", oldObj)
				}
				newE, ok := newObj.(*api.Secret)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Secret", newObj)
				}

				watcher.UpdateSecret(oldE, newE)
			},
		},
	)
	controller.Run(util.NeverStop)
}

func FindSelfPodIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ips := []net.IP{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {

			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			ips = append(ips, ip)
		}
	}

	if len(ips) == 0 {
		return nil, nil
	}

	if len(ips) > 1 {
		glog.Warning("Found multiple local IPs, making arbitrary choice: ", ips)
	}
	return ips[0], nil
}

// Gets the pod that we are running in; returns an error if it cannot be found
func (k *Kubernetes) GetSelfPod() (*api.Pod, error) {
	glog.Info("Querying kubernetes for self-pod")

	podIP, err := FindSelfPodIP()
	if err != nil {
		return nil, err
	}
	if podIP == nil {
		return nil, fmt.Errorf("cannot determine pod ip")
	}

	// We wait and retry in case of a delay before the pod is sent to the API
	attempt := 0
	for {
		pod, err := k.FindPodByPodIp(podIP.String())
		if err != nil {
			return nil, err
		}
		if pod != nil {
			return pod, nil
		}

		if attempt > 10 {
			return nil, fmt.Errorf("could not find self-pod in kubernetes API")
		}

		glog.Warning("Did not find self-pod; will wait and retry")
		time.Sleep(5 * time.Second)
	}
}

func (k *Kubernetes) FindSecret(namespace string, name string) (*api.Secret, error) {
	secret, err := k.kubeClient.Secrets(namespace).Get(name)
	if err != nil {

		apiStatusErr, ok := err.(kclient.APIStatus)
		if ok {
			status := apiStatusErr.Status()
			if status.Reason == unversioned.StatusReasonNotFound {
				return nil, nil
			}

			glog.V(2).Info("got APIStatus err: ", status)
		}

		return nil, err
	}
	return secret, nil
}

func (k *Kubernetes) CreateSecret(secret *api.Secret) (*api.Secret, error) {
	secret, err := k.kubeClient.Secrets(secret.Namespace).Create(secret)
	return secret, err
}

func (k *Kubernetes) FindPodByPodIp(podIP string) (*api.Pod, error) {
	// TODO: make this efficient
	glog.Warningf("Querying kubernetes for pod by podIP is inefficient %q", podIP)

	// TODO: Can we use api.NamespaceAll,?
	pods, err := k.kubeClient.Pods(api.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	for j := range pods.Items {
		pod := &pods.Items[j]
		if pod.Status.PodIP == podIP {
			return pod, nil
		}
	}

	//	namespaces, err := k.kubeClient.Namespaces().List(labels.Everything(), fields.Everything())
	//	if err != nil {
	//		return nil, err
	//	}
	//	for i := range namespaces.Items {
	//		namespace := &namespaces.Items[i]
	//		pods, err := k.kubeClient.Pods(namespace.Name).List(labels.Everything(), fields.Everything())
	//		if err != nil {
	//			return nil, err
	//		}
	//
	//		for j := range pods.Items {
	//			pod := &pods.Items[j]
	//			if pod.Status.PodIP == podIP {
	//				return pod, nil
	//			}
	//		}
	//	}

	glog.Warningf("Unable to find pod by podIP=%q", podIP)

	return nil, nil
}

// TODO: Return map of ClusterMember (PV/PVC/Pod)?
func (k *KopePod) GetClusterMap(clusterID string) (map[string]*KopePod, error) {
	namespace := k.Pod.Namespace

	//	pvs, err := k.KubernetesClient.kubeClient.PersistentVolumes().List(labels.NewRequirement("kope.io/clusterid", labels.InOperator, util.NewStringSet(clusterID)), fields.Everything())
	//	if err != nil {
	//		return nil, err
	//	}

	filter := labels.Everything().Add("kope.io/clusterid", labels.InOperator, []string{clusterID})
	pvcs, err := k.KubernetesClient.kubeClient.PersistentVolumeClaims(namespace).List(filter, fields.Everything())
	if err != nil {
		return nil, err
	}

	pods, err := k.KubernetesClient.kubeClient.Pods(namespace).List(filter, fields.Everything())
	if err != nil {
		return nil, err
	}

	//	pvMap := map[string]string{}
	//	for i := range pvs.Items {
	//		pv := &pvs.Items[i]
	//		nodeID := pv.Labels["kope.io/nodeid"]
	//		if nodeID == "" {
	//			continue
	//		}
	//		pvMap[pv.Name] = nodeID
	//	}

	clusterPods := map[string]*KopePod{}

	pvcMap := map[string]string{}
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		glog.Info("PVC", pvc)
		nodeID := pvc.Labels["kope.io/nodeid"]
		//		if nodeID == "" {
		//			if pvc.Spec.VolumeName != "" {
		//				nodeID, _ = pvMap[pvc.Spec.VolumeName]
		//			}
		//		}
		if nodeID == "" {
			continue
		}
		pvcMap[pvc.Name] = nodeID

		// If the pod is not found, we still want the cluster map to record the nodeid
		clusterPods[nodeID] = nil
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		glog.Info("POD", pod)
		for j := range pod.Spec.Volumes {
			volume := &pod.Spec.Volumes[j]
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				nodeId, _ := pvcMap[pvcName]
				if nodeId != "" {
					kopePod := &KopePod{}
					kopePod.Pod = pod
					kopePod.KubernetesClient = k.KubernetesClient
					clusterPods[nodeId] = kopePod
				}
			}
		}
	}

	return clusterPods, nil
}

type KopePod struct {
	KubernetesClient *Kubernetes
	Pod              *api.Pod

	volumes []*KopeVolume
}

func (p *KopePod) MemoryLimit() (int64, bool) {
	if p.Pod != nil {
		// Kubernetes
		if len(p.Pod.Spec.Containers) > 1 {
			glog.Warning("Found multiple containers in pod, choosing arbitrarily")
		}
		memoryLimit := p.Pod.Spec.Containers[0].Resources.Limits.Memory()
		if memoryLimit != nil {
			memoryLimitBytes := memoryLimit.Value()
			if memoryLimitBytes > 0 {
				return memoryLimitBytes, true
			}
		}
	}

	return 0, false
}

func (p*KopePod) Label(key string) (string, bool) {
	if p.Pod != nil {
		// Kubernetes
		labels := p.Pod.Labels
		v, found :=  labels["kope.io/" + key]
		return v, found
	}

	return "", false
}

func (p *KopePod) GetVolumes() ([]*KopeVolume, error) {
	kopeVolumes := p.volumes
	if kopeVolumes == nil {
		kopeVolumes = []*KopeVolume{}
		volumes := p.Pod.Spec.Volumes
		for i := range volumes {
			volume := &volumes[i]
			kopeVolume := &KopeVolume{}
			kopeVolume.pod = p
			kopeVolume.volume = volume
			kopeVolumes = append(kopeVolumes, kopeVolume)
		}
		p.volumes = kopeVolumes
	}
	return kopeVolumes, nil
}

type KopeVolume struct {
	pod    *KopePod
	volume *api.Volume

	pvc *api.PersistentVolumeClaim
	pv  *api.PersistentVolume
}

func (v *KopeVolume) GetPersistentVolumeClaim() (*api.PersistentVolumeClaim, error) {
	client := v.pod.KubernetesClient.kubeClient

	pvc := v.pvc
	if pvc == nil {
		var err error
		if v.volume.PersistentVolumeClaim != nil {
			claimName := v.volume.PersistentVolumeClaim.ClaimName
			pvc, err = client.PersistentVolumeClaims(v.pod.Pod.Namespace).Get(claimName)
			if err != nil {
				return nil, err
			}
			v.pvc = pvc
		}
	}
	return pvc, nil
}

func (v *KopeVolume) GetPersistentVolume() (*api.PersistentVolume, error) {
	client := v.pod.KubernetesClient.kubeClient

	pv := v.pv
	if pv == nil {
		pvc, err := v.GetPersistentVolumeClaim()
		if err != nil {
			return nil, err
		}

		if pvc.Spec.VolumeName != "" {
			pv, err = client.PersistentVolumes().Get(pvc.Spec.VolumeName)
			if err != nil {
				return nil, err
			}
			v.pv = pv
		}
	}
	return pv, nil
}
