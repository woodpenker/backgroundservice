package backgroundservice_test

import (
	"backgroundservice"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/go-playground/assert"
)

var (
	flags = backgroundservice.Flags{
		BinPath: "nc",
		LogPath: "run.log",
		Args:    []string{"-l", "9999"},
	}
	lock = sync.Mutex{}
)

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
	lock.Lock()
	defer lock.Unlock()
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	err := am.Stop()
	assert.Equal(t, backgroundservice.ErrIsNotRunning, err)
	e := am.Start()
	assert.Equal(t, nil, e)
	t.Cleanup(func() {
		am.Stop()
		os.Remove("run.log")
	})
	err = am.Start()
	assert.NotEqual(t, nil, err)
	assert.Equal(t, "exec: already started", err.Error())
	err = am.Stop()
	assert.Equal(t, nil, err)
}

func TestStartParallel(t *testing.T) {
	lock.Lock()
	defer lock.Unlock()
	var wg sync.WaitGroup
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	t.Cleanup(func() {
		am.Stop()
		os.Remove("run.log")
	})
	am.Start()
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			e := am.Start()
			assert.NotEqual(t, nil, e)
			assert.Equal(t, "exec: already started", e.Error())
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestStopParallel(t *testing.T) {
	lock.Lock()
	defer lock.Unlock()
	var wg sync.WaitGroup
	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	t.Cleanup(func() {
		am.Stop()
		os.Remove("run.log")
	})
	am.Start()
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
	wg.Wait()
}
