package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"

	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"github.com/kopeio/kope/user"
	"github.com/kopeio/kope/utils"
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

type PostgresSecretData struct {
	User     string `json: "user"`
	Password string `json: "password"`
}

func (m *Manager) Configure() error {
	err := m.KopeBaseManager.Configure()
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

func (m *Manager) findSecretData(secretName string) (*PostgresSecretData, error) {
	me, err := m.GetSelfPod()
	if err != nil {
		return nil, err
	}

	secret, err := m.KubernetesClient.FindSecret(me.Pod.Namespace, secretName)
	if err != nil {
		return nil, chained.Error(err, "error fetching secret")
	}

	if secret == nil {
		return nil, nil
	}

	if secret.Data == nil {
		return nil, nil
	}

	configData, found := secret.Data["config.json"]
	if !found {
		glog.Warning("Secret found, but config.json not found")
		return nil, nil
	}

	config := &PostgresSecretData{}
	err = json.Unmarshal(configData, config)
	if err != nil {
		return nil, chained.Error(err, "error reading config.json")
	}

	return config, nil
}

func (m *Manager) writeSecretData(secretName string, config *PostgresSecretData) error {
	j, err := json.Marshal(config)
	if err != nil {
		return chained.Error(err, "error building secret config.json")
	}

	me, err := m.GetSelfPod()
	if err != nil {
		return err
	}

	secret := &api.Secret{}
	secret.Namespace = me.Pod.Namespace
	secret.Name = secretName
	secret.Type = "Opaque"
	secret.Data = map[string][]byte{}
	secret.Data["config.json"] = j

	_, err = m.KubernetesClient.CreateSecret(secret)
	if err != nil {
		return chained.Error(err, "error creating secret")
	}

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

		secretName := m.ClusterID
		if secretName == "" {
			secretName = "postgres"
		}

		config, err := m.findSecretData(secretName)
		if err != nil {
			return chained.Error(err, "error reading secret data")
		}

		if config == nil {
			config = &PostgresSecretData{}
			config.User = "postgres"
			password, err := utils.GeneratePassword(128)
			if err != nil {
				return chained.Error(err, "error generating password")
			}
			config.Password = password
			err = m.writeSecretData(secretName, config)
			if err != nil {
				return chained.Error(err, "error writing secret data")
			}
		}

		err = m.setRootPassword(config.Password)
		if err != nil {
			return chained.Error(err, "error setting root password")
		}
	}

	err = m.writeConfig()
	if err != nil {
		return chained.Error(err, "error writing configuration")
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
	return m.start()
}

func (m *Manager) start(extraArgs ...string) (*process.Process, error) {
	argv := []string{"/usr/lib/postgresql/9.4/bin/postgres"}
	argv = append(argv, "-D", m.config.DataDir)
	if len(extraArgs) != 0 {
		argv = append(argv, extraArgs...)
	}

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

func (m *Manager) writeConfig() error {
	pathPgHbaConf := path.Join(m.config.DataDir, "pg_hba.conf")
	err := kope.WriteTemplate(pathPgHbaConf, &m.config)
	if err != nil {
		return err
	}

	pathPostgresqlConf := path.Join(m.config.DataDir, "postgresql.conf")
	err = kope.WriteTemplate(pathPostgresqlConf, &m.config)
	if err != nil {
		return err
	}

	return nil
}

func sqlEscape(s string) string {
	return strings.Replace(s, "'", "''", -1)
}

func (m *Manager) setRootPassword(password string) error {
	// Start but only listen on UNIX pipes
	glog.Info("Starting postgres (listening locally only)")
	process, err := m.start("-c", "listen_addresses=")
	if err != nil {
		return chained.Error(err, "error starting postgres while trying to set root password")
	}

	go func() {
		glog.Info("waiting for exit")
		state, err := process.Wait()
		if err != nil {
			glog.Info("postgres exited with error condition: ", err)
		}
		if !state.Success() {
			glog.Info("postgres exited with non-zero exit-code")
		}

	}()

	err = m.waitHealthy(10 /*120*/ * time.Second)
	if err != nil {
		return chained.Error(err, "timeout waiting for postgres to start listening")
	}

	sql := "ALTER ROLE postgres WITH PASSWORD '" + sqlEscape(password) + "'"
	_, err = m.runPsql(sql)
	if err != nil {
		return chained.Error(err, "error running psql to change root password")
	}

	glog.Info("Stopping postgres")
	err = m.pgCtlStop()
	if err != nil {
		return nil
	}
	return nil
}

func (m *Manager) waitHealthy(timeout time.Duration) error {
	timeoutAt := time.Now().Add(timeout)
	for {
		if !time.Now().Before(timeoutAt) {
			return fmt.Errorf("postgres did not become ready before timeout")
		}
		if m.isHealthy() {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (m *Manager) pgCtlStop() error {
	argv := []string{"/usr/lib/postgresql/9.4/bin/pg_ctl", "stop", "-D", m.config.DataDir}

	_, err := m.runAsPostgresUser(argv)
	if err != nil {
		return chained.Error(err, "error stopping postgres")
	}

	return nil
}

func (m *Manager) isHealthy() bool {
	_, err := m.runPsql("SELECT 1")
	if err != nil {
		glog.V(2).Info("postgres not yet healthy: ", err)
		return false
	}
	return true
}

func (m *Manager) runPsql(sql string) (*os.ProcessState, error) {
	argv := []string{"/usr/lib/postgresql/9.4/bin/psql", "--username", "postgres", "-c", sql}
	argv = append(argv, "-h", "/var/run/postgresql")

	state, err := m.runAsPostgresUser(argv)
	if err != nil {
		return nil, err
	}
	if state.Success() {
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected exit code from psql")
}

func (m *Manager) runAsPostgresUser(argv []string) (*os.ProcessState, error) {
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

	return process.Wait()
}

func (m *Manager) runInitdb() error {
	postgresUser, err := user.Find("postgres")
	if err != nil {
		return chained.Error(err, "error finding user")
	}

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

	argv := []string{"/usr/lib/postgresql/9.4/bin/initdb"}
	argv = append(argv, "-D", m.config.DataDir)

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(postgresUser)
	config.Env = []string{"LANG=en_US.utf8"}
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
