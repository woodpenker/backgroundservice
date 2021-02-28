This is a package for create a deamon running background using some cmd binary commands.

---
Example: Create a http server listening at 8080 with a background  service nc listening at port 9999 and log to run.log. 
If the http server exists, the nc service exists too.

```go
var flags = backgroundservice.Flags{
	BinPath: "nc",
	LogPath: "run.log",
	Args:    []string{"-l", "9999"},
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	go func() {
		err := am.Start()
		if err != nil {
			fmt.Println(err)
		}
	}()

	go func() {
		http.ListenAndServe(":8080", http.FileServer(http.Dir("/usr/share/doc")))
	}()

	<-stop
	err := am.Stop()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("after stop:", am.IsRunning())
	os.Remove("run.log")

}
```

---
Methods: 
```go
    Stop() error     // Stop stop the background service, if stop failed will return error: ErrNoCmd, ErrIsNotRunning, ErrServerExit. This func call will not block. if call to this function returned error, the background service may still running
	Start() error    // Start **block function**, calling to this function will block until the server exit or being stopped. the log will be output to the LogPath defined by Flags. If the server start failed, error will be returned, but if the server is stopped by Stop(), no error will be returned. errors will be one of ErrIsRunning and ErrServerExit
	IsRunning() bool // IsRunning return whether the server is running at the time calling to this function. Note that this state may be changed after this calling returned.
    New(opts ...Opt) Service     // New return an new background service. If no flags is specified, `nc` will be used default as BinPath and `run.log` will be used default LogPath, and Args will be `-l 9999`
    WithFlags(flags Flags) Opt // WithFlags set the Flags for the cmd. ** Panic ** This will panic if no flags.BinPath is specified. If no LogPath is specified, use run.log by default
```