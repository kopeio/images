package process

import (
	"github.com/golang/glog"
	"github.com/kopeio/kope/user"
	"os"
	"strings"
	"syscall"
)

type ProcessConfig struct {
	Argv []string

	Dir string
	Env []string

	Credential *syscall.Credential
}

type Process struct {
	process *os.Process
}

func (p *ProcessConfig) Start() (*Process, error) {
	argv := p.Argv
	name := argv[0]

	attr := &os.ProcAttr{}
	attr.Dir = p.Dir
	attr.Env = p.Env

	attr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	attr.Sys = &syscall.SysProcAttr{}
	if p.Credential != nil {
		attr.Sys.Credential = p.Credential
	}

	glog.Info("Running: ", strings.Join(argv, " "))
	process, err := os.StartProcess(name, argv, attr)

	if err != nil {
		return nil, err
	}

	proc := &Process{}
	proc.process = process
	return proc, nil
}

func (p *Process) Wait() (*os.ProcessState, error) {
	return p.process.Wait()
}

func (p *ProcessConfig) SetCredential(user *user.User) {
	p.Credential = &syscall.Credential{}
	p.Credential.Uid = uint32(user.Uid)
	p.Credential.Gid = uint32(user.Gid)
}
