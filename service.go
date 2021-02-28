package backgroundservice

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	// ErrNoCmd indicate that the cmd is not correctly
	ErrNoCmd = errors.New("the cmd is not correctly")
	// ErrIsRunning indicate that call start func twice parallel
	ErrIsRunning = errors.New("the service is running, do not need to started it twice")
	// ErrIsNotRunning indicate that server is not started yet
	ErrIsNotRunning = errors.New("the service is not running")
	// ErrServerExit indicate that server is exit background without using stop
	ErrServerExit = errors.New("service exit background without using stop operation")
)

// Service is the long running background cmd service
type Service interface {
	Stop() error     // Stop stop the background service, if stop failed will return error: ErrNoCmd, ErrIsNotRunning, ErrServerExit. This func call will not block. if call to this function returned error, the background service may still running
	Start() error    // Start **block function**, calling to this function will block until the server exit or being stopped. the log will be output to the LogPath defined by Flags. If the server start failed, error will be returned, but if the server is stopped by Stop(), no error will be returned. errors will be one of ErrIsRunning and ErrServerExit
	IsRunning() bool // IsRunning return whether the server is running at the time calling to this function. Note that this state may be changed after this calling returned.
}

const (
	stopped     uint32 = 0
	running     uint32 = 1
	restarting  uint32 = 2
	unknownStop uint32 = 0
	optStop     uint32 = 1
)

type service struct {
	state     uint32 // 0 not running, 1 running, 2 restarting
	stopByOpt uint32 // 1 is stopped by operation, 0 other reason stopped it
	err       error
	flags     Flags
	cmd       *exec.Cmd
	m         sync.Mutex
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

func (a *service) run() error {
	// Run the cmd and wait for it to exit
	// Check if it is stopped by Stop(), no matter what err returned by Run()
	a.cmd.Run()
	a.m.Lock()
	a.state = stopped
	a.m.Unlock()
	// If is stopped by Stop(), return no error
	// If a.stopByOpt is unknownStop, return the error to indicate that this
	// service is stopped by other reason
	if atomic.CompareAndSwapUint32(&a.stopByOpt, unknownStop, unknownStop) {
		return ErrServerExit
	}
	return nil
}

// Start start the service,
// only allowed one service instance exist background
func (a *service) Start() error {
	// Check if service exist, if so return error
	if atomic.CompareAndSwapUint32(&a.state, running, running) {
		return ErrIsRunning
	}
	a.m.Lock()
	if a.state == running {
		a.m.Unlock()
		return ErrIsRunning
	}
	a.state = running
	a.stopByOpt = unknownStop
	a.m.Unlock()
	return a.run()
}

func (a *service) stop() error {
	// The server's state is running now, check if the process and cmd exists
	if a.cmd == nil {
		return ErrNoCmd
	}
	// If cmd is not started, return ErrIsNotRunning error
	if a.cmd.Process == nil {
		return ErrIsNotRunning
	}
	// No matter what happened blew, the server's stopByOpt
	// state will be tagged as optStop
	defer func() {
		if r := recover(); r != nil {
			// panic so that Process.pid is nil, so process is not running
			a.state = stopped
		}
	}()
	// Process.pid may be nil, so panic may happened here,
	// so a defer recover defined
	pgid, err := syscall.Getpgid(a.cmd.Process.Pid)
	if err != nil {
		// if get pgid error,then set the service's state to stopped
		// and return the error
		a.state = stopped
		return err
	}
	a.stopByOpt = optStop
	err = syscall.Kill(-pgid, syscall.SIGTERM)
	if err != nil {
		// if kill -15 failed then try kill - 9 and
		// return the error if it failed again
		err = syscall.Kill(-pgid, syscall.SIGKILL)
		if err != nil {
			// the process may not exist, so reset stopByOpt
			a.stopByOpt = unknownStop
			return err
		}
	}
	// Now service is exited, set the service' state to
	// stopped and return no error
	a.state = stopped
	return nil
}

func (a *service) Stop() error {
	// Check if the cmd is stopped, if so return error
	if atomic.CompareAndSwapUint32(&a.state, stopped, stopped) {
		return ErrIsNotRunning
	}
	// avoid kill twice or other process
	a.m.Lock()
	defer a.m.Unlock()
	if a.state == stopped {
		return ErrIsNotRunning
	}
	return a.stop()
}

func (a *service) IsRunning() bool {
	return atomic.LoadUint32(&a.state) != stopped
}
