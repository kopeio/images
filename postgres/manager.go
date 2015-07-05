package postgres

import (
	"errors"
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"github.com/kopeio/kope/user"
	"os"
	"time"
)

const DefaultMemory = 128

type Manager struct {
	base.KopeBaseManager
	process *process.Process
	config  Config
}

type Config struct {
	DataDir  string
	MemoryMB int
}

func (m *Manager) Configure() error {
	err := m.ConfigureMemory()
	if err != nil {
		return err
	}

	if m.MemoryMB == 0 {
		m.MemoryMB = DefaultMemory
	} else {
		memoryLimitMB := m.MemoryMB

		// We leave 32 MB for overhead (connections etc)
		memoryLimitMB -= 32

		if memoryLimitMB < 0 {
			glog.Warning("Memory limit was too low; ignoring")
			m.MemoryMB = DefaultMemory
		} else {
			glog.Info("Setting postgres memory to ", memoryLimitMB)
			m.MemoryMB = memoryLimitMB
		}
	}

	m.config.DataDir = "/data/db"
	m.config.MemoryMB = m.MemoryMB

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

	if !kope.FileExists(m.config.DataDir) {
		err = m.runInitdb()
		if err != nil {
			return chained.Error(err, "error initializing database")
		}
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
	argv := []string{"/usr/lib/postgresql/9.4/bin/postgres"}
	argv = append(argv, "-D", m.config.DataDir)

	postgresUser, err := user.Find("postgres")
	if err != nil {
		return nil, chained.Error(err, "error finding user")
	}

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(postgresUser)

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}

func (m *Manager) runInitdb() error {
	argv := []string{"/usr/lib/postgresql/9.4/bin/initdb"}
	argv = append(argv, "-D", m.config.DataDir)

	postgresUser, err := user.Find("postgres")
	if err != nil {
		return chained.Error(err, "error finding user")
	}

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(postgresUser)

	for _, dir := range []string{m.config.DataDir} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return chained.Error(err, "error doing mkdir on: ", dir)
		}
		err = postgresUser.Chown(dir)
		if err != nil {
			return chained.Error(err, "error doing chown on: ", dir)
		}
	}

	process, err := config.Start()
	if err != nil {
		return chained.Error(err, "error starting initdb")
	}
	result, err := process.Wait()
	if err != nil {
		return chained.Error(err, "error calling initdb")
	}
	if !result.Success() {
		glog.Warning("initdb failed: ", result)
		return errors.New("initdb failed")
	}
	return nil
}
