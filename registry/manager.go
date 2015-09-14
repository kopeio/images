package registry

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"k8s.io/kubernetes/pkg/api"

	"golang.org/x/crypto/bcrypt"

	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"github.com/kopeio/kope/user"
	"github.com/kopeio/kope/utils"
)

// cost parameter for bcrypting hashing when generating htpasswd
const BcryptCost = 11

type Manager struct {
	base.KopeBaseManager

	secretName string
	serverName string
	dataDir    string
	process    *process.Process
	config     ConfigData
}

type ConfigData struct {
	HtpasswdPath string
	RegistryDir  string
	Secret       string
}

type DockerConfig struct {
	Servers map[string]*DockerServerConfig
}

type DockerServerConfig struct {
	Email string `json:"email"`
	Auth  string `json:"auth"`
}

func (m *Manager) findDockerAuth() (*DockerServerConfig, error) {
	me, err := m.GetSelfPod()
	if err != nil {
		return nil, err
	}

	secret, err := m.KubernetesClient.FindSecret(me.Pod.Namespace, m.secretName)
	if err != nil {
		return nil, chained.Error(err, "error fetching secret")
	}

	if secret == nil {
		return nil, nil
	}

	if secret.Data == nil {
		return nil, nil
	}

	dockercfg, found := secret.Data[".dockercfg"]
	if !found {
		glog.Warning("Secret found, but .dockercfg not found")
		return nil, nil
	}

	dockerConfig := &DockerConfig{}
	dockerConfig.Servers = map[string]*DockerServerConfig{}
	err = json.Unmarshal(dockercfg, &dockerConfig.Servers)
	if err != nil {
		return nil, chained.Error(err, "error reading .dockercfg")
	}

	if len(dockerConfig.Servers) > 1 {
		// No way to make a sensible choice
		return nil, fmt.Errorf("multiple servers found in docker registry secret")
	}

	for k := range dockerConfig.Servers {
		return dockerConfig.Servers[k], nil
	}

	glog.Warning("no servers found in .dockercfg")
	return nil, nil
}

func (m *Manager) writeDockerAuth(serverName string, config *DockerServerConfig) error {
	dockerConfig := &DockerConfig{}
	dockerConfig.Servers = map[string]*DockerServerConfig{}
	dockerConfig.Servers[serverName] = config

	j, err := json.Marshal(&dockerConfig.Servers)
	if err != nil {
		return chained.Error(err, "error building .dockercfg")
	}

	me, err := m.GetSelfPod()
	if err != nil {
		return err
	}

	secret := &api.Secret{}
	secret.Namespace = me.Pod.Namespace
	secret.Name = m.secretName
	secret.Type = "kubernetes.io/dockercfg"
	secret.Data = map[string][]byte{}
	secret.Data[".dockercfg"] = j

	_, err = m.KubernetesClient.CreateSecret(secret)
	if err != nil {
		return chained.Error(err, "error creating secret")
	}

	return nil
}

func (m *Manager) writeHtpasswd(path string) error {
	dockerServerConfig, err := m.findDockerAuth()
	if err != nil {
		return err
	}

	if dockerServerConfig == nil {
		glog.Info("Generating new credentials")

		password, err := utils.GeneratePassword(128)
		if err != nil {
			return err
		}

		username := "docker"
		email := "not@val.id"

		dockerServerConfig = &DockerServerConfig{}
		dockerServerConfig.Email = email
		dockerServerConfig.Auth = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

		err = m.writeDockerAuth(m.serverName, dockerServerConfig)
		if err != nil {
			return err
		}
	} else {
		glog.Info("Using existing credentials")
	}

	authBytes, err := base64.StdEncoding.DecodeString(dockerServerConfig.Auth)
	if err != nil {
		return fmt.Errorf("unable to decode docker auth")
	}

	colonIndex := bytes.IndexByte(authBytes, ':')
	if colonIndex == -1 {
		return fmt.Errorf("unable to interpret docker auth")
	}

	username := authBytes[0:colonIndex]
	password := authBytes[colonIndex+1:]

	bcryptPassword, err := bcrypt.GenerateFromPassword(password, BcryptCost)
	if err != nil {
		return chained.Error(err, "error bcrypting password")
	}

	var buffer bytes.Buffer
	buffer.Write(username)
	buffer.WriteString(":")
	buffer.Write(bcryptPassword)
	buffer.WriteString("\n")

	err = ioutil.WriteFile(path, buffer.Bytes(), 0700)
	if err != nil {
		return chained.Error(err, "error writing htpasswd file ", path)
	}
	return nil
}

func (m *Manager) Configure() error {
	// TODO: Merge Init and Config?
	err := m.Init()
	if err != nil {
		return err
	}

	err = m.KopeBaseManager.Configure()
	if err != nil {
		return err
	}

	m.secretName = "docker-registry"
	// TODO: Fetch from service instead?
	m.serverName = os.Getenv("SERVER_NAME")
	if m.serverName == "" {
		return fmt.Errorf("Must set SERVER_NAME")
	}

	m.dataDir = "/data"
	m.config.RegistryDir = path.Join(m.dataDir, "registry")

	htpasswdPath := path.Join(m.dataDir, "htpasswd")
	err = m.writeHtpasswd(htpasswdPath)
	if err != nil {
		return err
	}
	m.config.HtpasswdPath = htpasswdPath

	// Secret: saved in /data/secret, generate and save if not found
	secretPath := path.Join(m.dataDir, "secret")
	secretBytes, err := utils.ReadFileIfExists(secretPath)
	if err != nil {
		return err
	}
	if secretBytes == nil {
		secretBytes = make([]byte, 16)
		_, err := crypto_rand.Read(secretBytes)
		if err != nil {
			return chained.Error(err, "error generating secret")
		}
		err = ioutil.WriteFile(secretPath, secretBytes, 0700)
		if err != nil {
			return chained.Error(err, "error writing secret file")
		}
	}

	// TODO: Get from k8s secret so it can be shared?
	m.config.Secret = base64.StdEncoding.EncodeToString(secretBytes)

	return nil
}

func (m *Manager) Manage() error {
	err := m.Configure()
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

	registryUser, err := user.Find("registry")
	if err != nil {
		return nil, chained.Error(err, "error finding user")
	}

	for _, dir := range []string{m.config.RegistryDir} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return nil, chained.Error(err, "error doing mkdir on: ", dir)
		}
		err = registryUser.Chown(dir)
		if err != nil {
			return nil, err
		}
	}

	err = registryUser.Chown(m.config.HtpasswdPath)
	if err != nil {
		return nil, err
	}

	argv := []string{"/opt/registry/registry"}
	argv = append(argv, configPath)

	config := &process.ProcessConfig{}
	config.Argv = argv
	config.SetCredential(registryUser)

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
}
