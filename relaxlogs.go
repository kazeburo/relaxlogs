package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	flags "github.com/jessevdk/go-flags"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

// Version by Makefile
var Version string

const relaxLoggerBufferSize = 1024 * 1024

const timeFormat = "02/Jan/2006:15:04:05 +0900"

type cmdOpts struct {
	LogDir       string `long:"log-dir" default:"" description:"Directory to store logfiles"`
	RotationTime int64  `long:"rotation-time" default:"60" description:"The time between log file rotations in minutes"`
	MaxAge       int64  `long:"max-age" default:"1440" description:"Maximum age of files (based on mtime), in minutes"`
	WithTime     bool   `long:"with-time" description:"Enable to prepend time"`
	Version      bool   `short:"v" long:"version" description:"Show version"`
}

// RelaxLogger bufio with lock
type RelaxLogger struct {
	sm        sync.Mutex
	tm        sync.RWMutex
	w         *bufio.Writer
	withTime  bool
	timestamp []byte
}

func newRelaxLogger(withTime bool, logDir string, rotationTime int64, maxAge int64) (*RelaxLogger, error) {
	if logDir == "stdout" || logDir == "" {
		return &RelaxLogger{w: bufio.NewWriterSize(os.Stdout, relaxLoggerBufferSize)}, nil
	}
	absLogDir, err := filepath.Abs(logDir)
	if err != nil {
		return nil, err
	}
	logFile := absLogDir
	linkName := absLogDir
	if !strings.HasSuffix(logDir, "/") {
		logFile += "/"
		linkName += "/"
	}
	logFile += "log.%Y%m%d%H%M"
	linkName += "current"

	logger, err := rotatelogs.New(
		logFile,
		rotatelogs.WithLinkName(linkName),
		rotatelogs.WithMaxAge(time.Duration(maxAge)*time.Minute),
		rotatelogs.WithRotationTime(time.Duration(rotationTime)*time.Minute),
	)
	if err != nil {
		return nil, err
	}

	bufsize := relaxLoggerBufferSize + 1
	if withTime {
		bufsize += len(timeFormat) + 3
	}

	rl := &RelaxLogger{
		w:        bufio.NewWriterSize(logger, bufsize),
		withTime: withTime,
	}
	rl.TimeTicker()
	return rl, nil
}

// Flush with lock
func (rl *RelaxLogger) Flush() {
	rl.sm.Lock()
	defer rl.sm.Unlock()
	rl.w.Flush()
}

// Write with lock
func (rl *RelaxLogger) Write(buf []byte) (int, error) {
	rl.sm.Lock()
	defer rl.sm.Unlock()
	bufLen := len(buf) + 1 //newline
	if rl.withTime {
		bufLen += len(timeFormat) + 3
	}

	if rl.w.Available() > 0 && bufLen > rl.w.Available() {
		rl.w.Flush()
	}
	timestampLen := 0
	var err error
	if rl.withTime {
		timestampLen, err = rl.w.Write(rl.getTimestamp())
		if err != nil {
			return timestampLen, err
		}
	}
	bodyLen, err := rl.w.Write(buf)
	bodyLen += timestampLen
	if err != nil {
		return bodyLen, err
	}

	err = rl.w.WriteByte('\n')
	if err != nil {
		return bodyLen, err
	}
	bodyLen++ // newline
	return bodyLen, err
}

// TimeTicker : run time updater
func (rl *RelaxLogger) TimeTicker() {
	rl.tm.Lock()
	rl.timestamp = []byte("[" + time.Now().Format(timeFormat) + "] ")
	rl.tm.Unlock()

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case _ = <-ticker.C:
				rl.tm.Lock()
				rl.timestamp = []byte("[" + time.Now().Format("02/Jan/2006:15:04:05 +0900") + "] ")
				rl.tm.Unlock()
			}
		}
	}()
}

func (rl *RelaxLogger) getTimestamp() []byte {
	rl.tm.RLock()
	defer rl.tm.RUnlock()
	return rl.timestamp
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

	rl, err := newRelaxLogger(opts.WithTime, opts.LogDir, opts.RotationTime, opts.MaxAge)
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
	stdin.Buffer(make([]byte, 10000), relaxLoggerBufferSize)
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
