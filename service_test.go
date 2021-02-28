package backgroundservice_test

import (
	"backgroundservice"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/assert"
)

var flags = backgroundservice.Flags{
	BinPath: "nc",
	LogPath: "run.log",
	Args:    []string{"-l", "9999"},
}

func TestWithFlags(t *testing.T) {
	cases := []struct {
		exp backgroundservice.Flags
		in  backgroundservice.Flags
	}{
		{
			exp: backgroundservice.Flags{
				BinPath: "../output/bin/alertmanager",
				LogPath: "../output/conf/backgroundservice.yml",
				Args:    []string{"-l", "9999"},
			},
			in: backgroundservice.Flags{
				BinPath: "../output/bin/alertmanager",
				LogPath: "../output/conf/backgroundservice.yml",
				Args:    []string{"-l", "9999"},
			},
		},
		{
			exp: backgroundservice.Flags{
				BinPath: "../output/bin/alertmanager",
				LogPath: "run.log",
				Args:    []string{"-l", "9999"},
			},
			in: backgroundservice.Flags{
				BinPath: "../output/bin/alertmanager",
				Args:    []string{"-l", "9999"},
			},
		},
		{
			exp: backgroundservice.Flags{
				BinPath: "bin/alertmanager",
				LogPath: "run.log",
				Args:    nil,
			},
			in: backgroundservice.Flags{
				BinPath: "bin/alertmanager",
			},
		},
		{
			exp: backgroundservice.Flags{
				BinPath: "nc",
				LogPath: "run.log",
				Args:    []string{"-l", "9999"},
			},
			in: backgroundservice.Flags{},
		},
	}
	for k, c := range cases {
		t.Run(fmt.Sprintf("@%v", k), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					assert.Equal(t, 3, k)
				}
			}()
			out := backgroundservice.WithFlags(c.in)()
			assert.Equal(t, c.exp, out)
		})
	}
}

func TestRunning(t *testing.T) {
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	err := am.Stop()
	assert.Equal(t, backgroundservice.ErrIsNotRunning, err)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		e := am.Start()
		assert.Equal(t, nil, e)
		wg.Done()
	}()
	defer func() {
		am.Stop()
		os.Remove("run.log")
	}()
	tick := time.NewTicker(1 * time.Second)
	<-tick.C
	err = am.Start()
	assert.Equal(t, backgroundservice.ErrIsRunning, err)
	assert.Equal(t, true, am.IsRunning())
	err = am.Stop()
	assert.Equal(t, nil, err)
	assert.Equal(t, false, am.IsRunning())
	wg.Wait()
}

func TestStartParallel(t *testing.T) {
	var wg sync.WaitGroup
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	defer func() {
		am.Stop()
		os.Remove("run.log")
	}()
	go func() {
		am.Start()
	}()
	<-time.After(1 * time.Second)
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			e := am.Start()
			assert.Equal(t, backgroundservice.ErrIsRunning, e)
			wg.Done()
		}()
	}
	<-time.After(1 * time.Second)
	assert.Equal(t, true, am.IsRunning())
	wg.Wait()
	assert.Equal(t, true, am.IsRunning())
}

func TestStopParallel(t *testing.T) {
	var wg sync.WaitGroup
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	defer func() {
		am.Stop()
		os.Remove("run.log")
	}()
	go func() {
		am.Start()
	}()
	<-time.After(1 * time.Second)
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			e := am.Stop()
			if e != nil {
				assert.Equal(t, backgroundservice.ErrIsNotRunning, e)
			}
			wg.Done()
		}()
	}
	<-time.After(1 * time.Second)
	assert.Equal(t, false, am.IsRunning())
	wg.Wait()
}
