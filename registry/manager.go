package registry

import (
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"time"
)

type Manager struct {
	base.KopeBaseManager

	process *process.Process
	config  ConfigData
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
	err := m.Init()
	if err != nil {
		return chained.Error(err, "error initializing")
	}

	err = m.Configure()
	if err != nil {
		return chained.Error(err, "error configuring")
	}

	process, err := m.Start()
	if err != nil {
		return chained.Error(err, "error starting")
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
