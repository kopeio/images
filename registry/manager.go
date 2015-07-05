package registry

import (
	"fmt"
	"github.com/kopeio/kope/process"
	"time"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/chained"
)

type Manager struct {
	process  *process.Process
	KubernetesClient *kope.Kubernetes
	config ConfigData
}

type ConfigData struct {
	Secret string
}

func (m *Manager) Configure() error {
	// TODO: Get from k8s secret?
	m.config.Secret = "somesecret"

	return nil
}

func (m *Manager) Manage() error {
	err := m.Configure()
	if err != nil {
		return fmt.Errorf("error configuring registry: %v", err)
	}

	process, err := m.Start()
	if err != nil {
		return fmt.Errorf("error starting registry: %v", err)
	}
	m.process = process

	for {
		time.Sleep(5 * time.Second)
	}

	return nil
}

func (m *Manager) Start() (*process.Process, error) {
	configPath := "/config.yml"
	err := kope.WriteTemplate(configPath, &m.config)
	if err != nil {
		return nil, chained.Error(err, "Error writing configuration template")
	}

	argv := []string{"/opt/registry/registry"}
	argv = append(argv, configPath)

	config := &process.ProcessConfig{}
	config.Argv = argv

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
