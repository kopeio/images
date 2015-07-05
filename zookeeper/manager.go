package zookeeper

import (
	"github.com/golang/glog"
	"github.com/kopeio/kope"
	"github.com/kopeio/kope/base"
	"github.com/kopeio/kope/chained"
	"github.com/kopeio/kope/process"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultMemory = 256

type Manager struct {
	base.KopeBaseManager
	process *process.Process
	config  Config
}

type ZkServer struct {
	Id         int
	Host       string
	ProxyPort  int
	LeaderPort int
}

type Config struct {
	Servers []ZkServer
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
			glog.Info("Setting memory to ", memoryLimitMB)
			m.MemoryMB = memoryLimitMB
		}
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
	for _, dir := range []string{"/data/conf", "/data/zk/logs", "/data/zk/data"} {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			return nil, chained.Error(err, "error doing mkdir on: ", dir)
		}
	}

	clusterMap, err := m.GetClusterMap()
	if err != nil {
		return nil, err
	}

	hosts := map[string]string{}
	hostPrefix := "cluster-zk-"

	m.config.Servers = []ZkServer{}
	for k, pod := range clusterMap {
		zkServer := ZkServer{}
		id, err := strconv.Atoi(k)
		if err != nil {
			glog.Warning("Ignoring cluster entries with invalid nodeid: ", k)
			continue
		}
		zkServer.Id = id
		host := hostPrefix + k
		zkServer.Host = host
		zkServer.ProxyPort = 2888
		zkServer.LeaderPort = 3888
		m.config.Servers = append(m.config.Servers, zkServer)

		podIP := ""
		if pod != nil {
			podIP = pod.Pod.Status.PodIP
		}
		hosts[host] = podIP
	}

	err = kope.SetEtcHosts(hostPrefix, hosts)
	if err != nil {
		return nil, err
	}
	err = kope.WriteTemplate("/data/conf/zoo.cfg", &m.config)
	if err != nil {
		return nil, err
	}
	err = kope.WriteTemplate("/data/conf/log4j.properties", &m.config)
	if err != nil {
		return nil, err
	}

	//export ZOOCFGDIR=/data/conf

	// TODO: Actually set memory

	argv := []string{"/usr/bin/java"}

	//java -Dzookeeper.log.dir=. -Dzookeeper.root.logger=INFO,CONSOLE -cp /opt/zk/bin/../build/classes:/opt/zk/bin/../build/lib/*.jar:/opt/zk/bin/../lib/slf4j-log4j12-1.6.1.jar:/opt/zk/bin/../lib/slf4j-api-1.6.1.jar:/opt/zk/bin/../lib/netty-3.7.0.Final.jar:/opt/zk/bin/../lib/log4j-1.2.16.jar:/opt/zk/bin/../lib/jline-0.9.94.jar:/opt/zk/bin/../zookeeper-3.4.6.jar:/opt/zk/bin/../src/java/lib/*.jar:/data/conf: -Dcom.sun.management.jmxremote -Dcom.sun.management.jmxremote.local.only=false org.apache.zookeeper.server.quorum.QuorumPeerMain /data/conf/zoo.cfg
	classpath := []string{"/opt/zk/bin/../build/classes", "/opt/zk/bin/../build/lib/*.jar", "/opt/zk/bin/../lib/slf4j-log4j12-1.6.1.jar", "/opt/zk/bin/../lib/slf4j-api-1.6.1.jar", "/opt/zk/bin/../lib/netty-3.7.0.Final.jar", "/opt/zk/bin/../lib/log4j-1.2.16.jar", "/opt/zk/bin/../lib/jline-0.9.94.jar", "/opt/zk/bin/../zookeeper-3.4.6.jar", "/opt/zk/bin/../src/java/lib/*.jar", "/data/conf"}
	argv = append(argv, "-cp", strings.Join(classpath, ":"))

	argv = append(argv, "-Dzookeeper.log.dir=.")
	argv = append(argv, "-Dzookeeper.root.logger=INFO,CONSOLE")
	argv = append(argv, "-Dcom.sun.management.jmxremote")
	argv = append(argv, "-Dcom.sun.management.jmxremote.local.only=false")
	argv = append(argv, "org.apache.zookeeper.server.quorum.QuorumPeerMain")
	argv = append(argv, "/data/conf/zoo.cfg")

	config := &process.ProcessConfig{}
	config.Argv = argv

	process, err := config.Start()
	if err != nil {
		return nil, err
	}
	return process, nil
	//echo "ruok" | nc 127.0.0.1 2181
}
