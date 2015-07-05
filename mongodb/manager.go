package mongodb

import (
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"github.com/kopeio/kope/user"
	"os"
	"time"
)

type Manager struct {
	base.KopeBaseManager
	process *process.Process
	config  Config
}

type Config struct {
	DataDir string
	LogDir  string
}

func (m *Manager) Configure() error {
	m.config.DataDir = "/data/db"
	m.config.LogDir = "/data/log"

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

	mongoUser, err := user.Find("mongodb")
	if err != nil {
		return nil, chained.Error(err, "error finding user")
	}

	for _, dir := range []string{m.config.DataDir, m.config.LogDir} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return nil, chained.Error(err, "error doing mkdir on: ", dir)
		}
		err = mongoUser.Chown(dir)
		if err != nil {
			return nil, err
		}
	}

	confPath := "/etc/mongod.conf"
	err = kope.WriteTemplate(confPath, &m.config)
	if err != nil {
		return nil, err
	}

	// TODO: Should we raise ulimit??

	argv := []string{"/opt/mongodb/bin/mongod"}
	argv = append(argv, "--config", confPath)

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(mongoUser)

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
