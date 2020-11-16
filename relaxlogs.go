package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/relaxlogs/logger"
)

// Version by Makefile
var Version string

const bufferSize = 1024 * 1024

type cmdOpts struct {
	LogDir       string `long:"log-dir" default:"" description:"Directory to store logfiles"`
	RotationTime int64  `long:"rotation-time" default:"60" description:"The time between log file rotations in minutes"`
	MaxAge       int64  `long:"max-age" default:"1440" description:"Maximum age of files (based on mtime), in minutes"`
	WithTime     bool   `long:"with-time" description:"Enable to prepend time"`
	Version      bool   `short:"v" long:"version" description:"Show version"`
}

func run() int {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		return 1
	}

	if opts.Version {
		fmt.Printf(`%s %s
Compiler: %s %s
`,
			os.Args[0],
			Version,
			runtime.Compiler,
			runtime.Version())
		return 0
	}

	rl, err := logger.NewRelaxLogger(opts.WithTime, bufferSize, opts.LogDir, opts.RotationTime, opts.MaxAge)
	if err != nil {
		log.Fatalf("failed initialize logger: %v", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	defer rl.Flush()
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			rl.Flush()
		}
	}()

	bufioChan := make(chan error, 1)

	stdin := bufio.NewScanner(os.Stdin)
	stdin.Buffer(make([]byte, 10000), bufferSize)
	go func() {
		for stdin.Scan() {
			_, err = rl.Write(stdin.Bytes())
			if err != nil {
				log.Fatal(err)
			}
		}
		bufioChan <- stdin.Err()
	}()

	exitCode := 0
loop:
	for {
		select {
		case err := <-bufioChan:
			if err != nil {
				log.Print(err)
				exitCode = 1
			}
			break loop
		case s := <-signalChan:
			log.Printf("Got signal: %s", s)
			exitCode = 1
			break loop
		}
	}

	return exitCode
}

func main() {
	os.Exit(run())
}
