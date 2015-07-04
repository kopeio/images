package kope

import (
	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller/framework"
	kcontrollerFramework "github.com/GoogleCloudPlatform/kubernetes/pkg/controller/framework"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"net"
	"net/url"
	"os"
	"time"
)

//argKubecfgFile         = flag.String("kubecfg_file", "", "Location of kubecfg file for access to kubernetes service")
//argKubeMasterUrl       = flag.String("kube_master_url", "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}", "Url to reach kubernetes master. Env variables in this flag will be expanded.")
//)

const DefaultKubecfgFile = "/etc/kubernetes/kubeconfig"

const (
// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second
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

type Kubernetes struct {
	kubeClient *kclient.Client
}

func IsKubernetes() bool {
	host := os.Getenv("KUBERNETES_HOST")
	return host != ""
}

func fileExists(path string) bool {
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
	return parsedUrl.String(), nil
}

func getKubeConfig(masterUrl string) (*kclient.Config, error) {
	s := "${KUBECFG_FILE}"
	p := os.ExpandEnv(s)

	if p == "" {
		if fileExists(DefaultKubecfgFile) {
			p = DefaultKubecfgFile
		}
	}

	if fileExists(p) {
		config, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&kclientcmd.ClientConfigLoadingRules{ExplicitPath: p},
			&kclientcmd.ConfigOverrides{ClusterInfo: kclientcmdapi.Cluster{Server: masterUrl}}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("error loading kubecfg file (%s): %v", p, err)
		}
		return config, nil
	} else {
		glog.Warning("No kubecfg file found; using default (empty) configuration")

		config := &kclient.Config{
			Host:    masterUrl,
			Version: "v1beta3",
		}
		return config, nil
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

	var serviceController *kcontrollerFramework.Controller
	_, serviceController = framework.NewInformer(
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
	serviceController.Run(util.NeverStop)
}

func (k *Kubernetes) WatchServices(watcher ServiceWatch) {
	glog.Info("Starting watch on k8s services")

	// Watch all changes
	// TODO: filter
	lw := cache.NewListWatchFromClient(k.kubeClient, "services", api.NamespaceAll, fields.Everything())

	var serviceController *kcontrollerFramework.Controller
	_, serviceController = framework.NewInformer(
		lw,
		&api.Endpoints{},
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
	serviceController.Run(util.NeverStop)
}

func findSelfPodIP() (net.IP, error) {
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

func (k *Kubernetes) FindSelfPod() (*api.Pod, error) {
	glog.Info("Querying kubernetes for self-pod")

	podIP, err := findSelfPodIP()
	if err != nil {
		return nil, err
	}
	if podIP == nil {
		return nil, nil
	}
	return k.FindPodByPodIp(podIP.String())
}

func (k *Kubernetes) FindPodByPodIp(podIP string) (*api.Pod, error) {
	// TODO: make this efficient
	glog.Warning("Querying kubernetes for self-pod is inefficient")


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
	return nil, nil
}

