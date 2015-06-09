package kope

import (
	"fmt"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller/framework"
	kcontrollerFramework "github.com/GoogleCloudPlatform/kubernetes/pkg/controller/framework"
	kSelector "github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
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
	AddEndpoints(e *kapi.Endpoints)
	DeleteEndpoints(e *kapi.Endpoints)
	UpdateEndpoints(oldEndpoints, newEndpoints *kapi.Endpoints)
}

type ServiceWatch interface {
	AddService(s *kapi.Service)
	DeleteService(s *kapi.Service)
	UpdateService(oldService, newService *kapi.Service)
}

type Kubernetes struct {
	kubeClient *kclient.Client
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
	masterUrl := parsedUrl.String()
	glog.V(2).Info("Using kubernetes master url: ", masterUrl)
	return masterUrl, nil
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
	lw := cache.NewListWatchFromClient(k.kubeClient, "endpoints", kapi.NamespaceAll, kSelector.Everything())

	var serviceController *kcontrollerFramework.Controller
	_, serviceController = framework.NewInformer(
		lw,
		&kapi.Endpoints{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				e, ok := o.(*kapi.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", o)
				} else {
					watcher.AddEndpoints(e)
				}
			},
			DeleteFunc: func(o interface{}) {
				e, ok := o.(*kapi.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", o)
				} else {
					watcher.DeleteEndpoints(e)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldE, ok := oldObj.(*kapi.Endpoints)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Endpoints", oldObj)
				}
				newE, ok := newObj.(*kapi.Endpoints)
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
	lw := cache.NewListWatchFromClient(k.kubeClient, "services", kapi.NamespaceAll, kSelector.Everything())

	var serviceController *kcontrollerFramework.Controller
	_, serviceController = framework.NewInformer(
		lw,
		&kapi.Endpoints{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				e, ok := o.(*kapi.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", o)
				} else {
					watcher.AddService(e)
				}
			},
			DeleteFunc: func(o interface{}) {
				e, ok := o.(*kapi.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", o)
				} else {
					watcher.DeleteService(e)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldE, ok := oldObj.(*kapi.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", oldObj)
				}
				newE, ok := newObj.(*kapi.Service)
				if !ok {
					glog.Warning("Got unexpected object of type %T, expecting Service", newObj)
				}

				watcher.UpdateService(oldE, newE)
			},
		},
	)
	serviceController.Run(util.NeverStop)
}
