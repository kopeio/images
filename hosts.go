package kope

import (
	"bytes"
	"github.com/golang/glog"
	"github.com/kopeio/kope/chained"
	"io/ioutil"
	"strings"
)

const EtcHostsPath = "/etc/hosts"

// This is reserved as TEST-NET-1 by rfc5737; unclear if we can do better
const NullRouteIp = "192.0.2.1"

func SetEtcHosts(prefix string, entries map[string]string) error {
	glog.Info("SetEtcHosts: ", entries)

	existing, err := ioutil.ReadFile(EtcHostsPath)
	if err != nil {
		return chained.Error(err, "error reading /etc/hosts")
	}

	done := map[string]bool{}

	var buffer bytes.Buffer
	for _, line := range strings.Split(string(existing), "\n") {
		write := line
		trimmed := strings.TrimSpace(line)
		tokens := strings.Split(trimmed, " ")
		if len(tokens) == 2 {
			if strings.HasPrefix(tokens[1], prefix) {
				ip, _ := entries[tokens[1]]
				if ip == "" {
					ip = NullRouteIp
				}
				done[tokens[1]] = true
				write = ip + " " + tokens[1]
			}
		}
		_, err := buffer.WriteString(write + "\n")
		if err != nil {
			return err
		}
	}

	for host, ip := range entries {
		if done[host] {
			continue
		}
		_, err := buffer.WriteString(ip + " " + host + "\n")
		if err != nil {
			return err
		}
	}

	// Because /etc/hosts is a bind-mount, we can't rename on top of it
	// TODO: We should deal with errors better here

	//	tempPath := EtcHostsPath + "." + strconv.FormatInt(time.Now().UnixNano(), 10)
	//
	//	err = ioutil.WriteFile(tempPath, buffer.Bytes(), 0544)
	//	if err != nil {
	//		_ = os.Remove(tempPath)
	//		return chained.Error(err, "error writing (temporary) /etc/hosts file")
	//	}
	//
	//	err = os.Rename(tempPath, EtcHostsPath)
	//	if err != nil {
	//		_ = os.Remove(tempPath)
	//		return chained.Error(err, "error renaming /etc/hosts temporary file")
	//	}

	newHosts := buffer.Bytes()
	glog.V(2).Info("Writing hosts ", string(newHosts))
	err = ioutil.WriteFile(EtcHostsPath, newHosts, 0544)
	if err != nil {
		return chained.Error(err, "error writing /etc/hosts file")
	}

	return nil
}
