package backgroundservice

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

var (
	// ErrNoCmd indicate that the cmd is not correctly
	ErrNoCmd = errors.New("the cmd is not correctly")
	// ErrIsNotRunning indicate that server is not started yet
	ErrIsNotRunning = errors.New("the service is not running")
)

// Service is the long running background cmd service
type Service interface {
	Stop() error  // Stop stop the background service, if stop failed will return error: ErrNoCmd, ErrIsNotRunning. This func call will not block. If call to this function returned error, the background service may still running
	Start() error // Start the log will be output to the LogPath defined by Flags. If the server start failed, error will be returned
	Wait() error  // Wait blocks until the background service exit. It return the error returned from cmd.Wait(). Calling to this function will block execution of Stop() and Start().
}

type service struct {
	flags Flags
	cmd   *exec.Cmd
	m     sync.Mutex
}

// Flags is the command line flags, which you want to start with
type Flags struct {
	BinPath string   // The binary path which you want to start background.
	Args    []string // The cmd arguments you want to pass
	LogPath string   // The log path where you want to save the logs of this binary cmd output
}

// Opt the func to set Flags for cmd
type Opt func() Flags

// WithFlags set the Flags for the cmd.
// ** Panic **
// This will panic if no flags.BinPath is specified
// If no LogPath is specified, use run.log by default
func WithFlags(flags Flags) Opt {
	return func() Flags {
		if flags.BinPath == "" {
			panic("no cmd binary path specified")
		}
		if flags.LogPath == "" {
			flags.LogPath = "run.log"
		}
		return flags
	}
}

// New return an new background service
// If no flags is specified, `nc` will be used default as BinPath and
// `run.log` will be used default LogPath, and Args will be `-l 9999`
func New(opts ...Opt) Service {
	flags := Flags{
		BinPath: "nc",
		LogPath: "run.log",
		Args:    []string{"-l", "9999"},
	}
	for _, o := range opts {
		flags = o()
		break
	}
	return new(flags)
}

func getCmd(flags Flags) *exec.Cmd {
	exe := flags.BinPath + " " + strings.Join(flags.Args, " ") + " &> '" + flags.LogPath + "'"
	cmd := exec.Command("bash", "-c", exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func new(flags Flags) *service {
	return &service{
		flags: flags,
		cmd:   getCmd(flags),
	}
}

func (a *service) start() error {
	err := a.cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

// Start start the service,
// only allowed one service instance exist background
func (a *service) Start() error {
	a.m.Lock()
	defer a.m.Unlock()
	return a.start()
}

func (a *service) stop() (err error) {
	// The server's state is running now, check if the process and cmd exists
	if a.cmd == nil {
		return ErrNoCmd
	}
	// If cmd is not started, return ErrIsNotRunning error
	if a.cmd.Process == nil {
		return ErrIsNotRunning
	}
	if a.cmd.Process.Pid <= 0 {
		return ErrIsNotRunning
	}
	// No matter what happened blew, the server's stopByOpt
	// state will be tagged as optStop
	defer func() {
		if r := recover(); r != nil {
			err = ErrIsNotRunning
		}
	}()
	// Process.pid may be negative, so panic may happened here,
	// so a defer recover defined
	pgid, err := syscall.Getpgid(a.cmd.Process.Pid)
	if err != nil {
		return err
	}
	err = syscall.Kill(-pgid, syscall.SIGTERM)
	if err != nil {
		// if kill -15 failed then try kill - 9 and
		// return the error if it failed again
		err = syscall.Kill(-pgid, syscall.SIGKILL)
		if err != nil {
			// the process may not exit
			return err
		}
	}
	return nil
}

func (a *service) Stop() error {
	a.m.Lock()
	defer a.m.Unlock()
	return a.stop()
}

func (a *service) Wait() error {
	a.m.Lock()
	defer a.m.Unlock()
	return a.Wait()
}
