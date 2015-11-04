package postgres

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"github.com/kopeio/kope/user"
	"github.com/kopeio/kope/utils"
	"io/ioutil"
	"k8s.io/kubernetes/pkg/api"
	"os"
	"path"
	"strings"
	"time"
)

const DefaultMemory = 128

type Manager struct {
	base.KopeBaseManager
	process   *process.Process
	config    Config
	SecretDir string
}

type Config struct {
	DataDir  string
	MemoryMB int
}

type PostgresSecretData struct {
	Db       string `json:"db,omitempty"`
	User     string `json:"user"`
	Password string `json:"password"`
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
	m.SecretDir = "/data/secrets"
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

	if config.User == "" && config.Db == "" && config.Password == "" {
		// Probably due to the format change
		return nil, fmt.Errorf("Secret data was unexpectedly empty")
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

	err = os.MkdirAll(m.SecretDir, 0777)
	if err != nil {
		return chained.Error(err, "error doing mkdir on: ", m.SecretDir)
	}
	secretPath := path.Join(m.SecretDir, secretName)
	err = ioutil.WriteFile(secretPath, j, 0700)
	if err != nil {
		return chained.Error(err, "error writing local secret file: "+secretPath)
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

		err = m.runInitdb()
		if err != nil {
			return chained.Error(err, "error initializing database")
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

	err = m.waitHealthy(120 * time.Second)
	if err != nil {
		return chained.Error(err, "timeout waiting for postgres to start listening")
	}

	glog.Info("Postgres is running")

	labels, err := m.GetLabels()
	if err != nil {
		return err
	}
	if labels != nil {
		appDB, _ := labels["db.kope.io/database"]
		appUser, _ := labels["db.kope.io/user"]
		if appDB != "" {
			if appUser == "" {
				appUser = appDB
			}
			secretName := "db-" + appDB
			err = m.ensureAppDb(secretName, appDB, appUser)
			if err != nil {
				return chained.Error(err, "error creating user db")
			}
		}
	}

	for {
		time.Sleep(5 * time.Second)
	}

	return nil
}

func (m *Manager) ensureAppDb(secretName string, db string, user string) error {
	glog.Infof("Ensuring that app db exists: db=%q, user=%q", db, user)
	config, err := m.findSecretData(secretName)
	if err != nil {
		return chained.Error(err, "error reading secret data")
	}

	if config == nil {
		config = &PostgresSecretData{}
		config.User = user
		config.Db = db
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

	_, err = m.ensureUser(config.User, config.Password)
	if err != nil {
		return chained.Error(err, "error creating user")
	}

	_, err = m.ensureDb(config.Db, config.User)
	if err != nil {
		return chained.Error(err, "error creating database")
	}

	return nil
}

func buildSql(sql string, args ...interface{}) string {
	format := strings.Replace(sql, "?", "%s", -1)
	escaped := []interface{}{} // Actually []string
	for _, arg := range args {
		var e string
		switch arg := arg.(type) {
		case bool:
			e = fmt.Sprintf("%t", arg)
		case int:
			e = fmt.Sprintf("%d", arg)
		case string:
			e = escapeSqlString(arg)
		default:
			glog.Fatalf("unexpected type in buildSql: %T\n", arg)
		}
		escaped = append(escaped, e)
	}
	return fmt.Sprintf(format, escaped...)
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		switch {
		case 'a' <= r && r <= 'z':
		case 'A' <= r && r <= 'Z':
		case '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return true
}

func escapeSqlString(s string) string {
	var buffer bytes.Buffer
	buffer.WriteString("'")
	for _, r := range s {
		switch {
		case 'a' <= r && r <= 'z':
			buffer.WriteRune(r)
		case 'A' <= r && r <= 'Z':
			buffer.WriteRune(r)
		case '0' <= r && r <= '9':
			buffer.WriteRune(r)
		default:
			switch r {
			case '_', '-', ' ':
				buffer.WriteRune(r)
			default:
				glog.Fatalf("unhandled character in escapeSqlString: %v", r)
			}
		}
	}
	buffer.WriteString("'")
	return string(buffer.Bytes())
}

func (m *Manager) ensureUser(user string, password string) (bool, error) {
	glog.Infof("Ensuring that user exists: %q", user)
	sql := buildSql("SELECT * FROM pg_catalog.pg_user WHERE usename=?", user)
	results, err := m.runPsql(sql)
	if err != nil {
		return false, chained.Error(err, "error querying for user")
	}

	if len(results.Rows) != 0 {
		return false, nil
	}

	// Output looks like 'CREATE ROLE'
	// Note that user is not escaped :-(
	if !isAlphanumeric(user) {
		return false, fmt.Errorf("invalid user name: %q", user)
	}
	glog.Infof("Creating user %q", user)
	sql = buildSql("CREATE USER "+user+" WITH PASSWORD ?", password)
	_, err = m.runPsql(sql)
	if err != nil {
		return false, chained.Error(err, "error creating user")
	}

	return true, nil
}

func (m *Manager) ensureDb(db string, owner string) (bool, error) {
	glog.Infof("Ensuring that database exists: %q", db)
	sql := buildSql("SELECT * FROM pg_catalog.pg_database WHERE datname=?", db)
	results, err := m.runPsql(sql)
	if err != nil {
		return false, chained.Error(err, "error querying for database")
	}

	if len(results.Rows) != 0 {
		return false, nil
	}

	// Note that db and owner are not escaped :-(
	if !isAlphanumeric(db) {
		return false, fmt.Errorf("invalid db name: %q", db)
	}
	if !isAlphanumeric(owner) {
		return false, fmt.Errorf("invalid user name: %q", owner)
	}
	glog.Infof("Creating database %q", db)
	sql = buildSql("CREATE DATABASE " + db + " WITH OWNER " + owner)
	_, err = m.runPsql(sql)
	if err != nil {
		return false, chained.Error(err, "error creating db")
	}

	return true, nil
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

	err = m.waitHealthy(120 * time.Second)
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

	_, _, err := m.runAsPostgresUser(argv)
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

func (m *Manager) runPsql(sql string) (*sqlResults, error) {
	argv := []string{"/usr/lib/postgresql/9.4/bin/psql", "--username", "postgres"}
	// Make parsable
	argv = append(argv, "--no-align", "-z", "--pset", "footer=off")
	argv = append(argv, "-c", sql)
	argv = append(argv, "-h", "/var/run/postgresql")

	stdout, stderr, err := m.runAsPostgresUser(argv)
	if err != nil {
		glog.Infof("error running sql query")
		glog.Infof("stdout: %s", stdout)
		glog.Infof("stderr: %s", stderr)
		return nil, chained.Error(err, "error running psql query")
	}

	if len(stderr) != 0 {
		glog.Warningf("unexpected stderr from psql: %q", stderr)
	}

	sqlOutput, err := parsePsqlOutput(stdout)
	if err != nil {
		return nil, err
	}
	return sqlOutput, nil
}

type sqlResults struct {
	Columns []string
	Rows    [][]string
}

func parsePsqlOutput(stdout string) (*sqlResults, error) {
	results := &sqlResults{}

	for i, line := range strings.Split(stdout, "\n") {
		if len(line) == 0 {
			continue
		}
		tokens := strings.Split(line, "\x00")
		if i == 0 {
			results.Columns = tokens
		} else {
			results.Rows = append(results.Rows, tokens)
		}
	}
	return results, nil
}

func (m *Manager) runAsPostgresUser(argv []string) (string, string, error) {
	postgresUser, err := user.Find("postgres")
	if err != nil {
		return "", "", chained.Error(err, "error finding user")
	}

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(postgresUser)

	return config.Exec()
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
