package main

import (
	"bufio"
	"fmt"
	"io"
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

var (
	Version string
)

type cmdOpts struct {
	LogDir       string `long:"log-dir" default:"" description:"Directory to store logfiles"`
	RotationTime int64  `long:"rotation-time" default:"60" description:"The time between log file rotations in minutes"`
	MaxAge       int64  `long:"max-age" default:"1440" description:"Maximum age of files (based on mtime), in minutes"`
	WithTime     bool   `long:"with-time" description:"Enable to prepend time"`
	Version      bool   `short:"v" long:"version" description:"Show version"`
}

func currentTime() []byte {
	return []byte(time.Now().Format("02/Jan/2006:15:04:05 +0900"))
}

func writeTo(logDir string, rotationTime int64, maxAge int64) (io.Writer, error) {
	if logDir == "stdout" || logDir == "" {
		return os.Stdout, nil
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

	return rotatelogs.New(
		logFile,
		rotatelogs.WithLinkName(linkName),
		rotatelogs.WithMaxAge(time.Duration(maxAge)*time.Minute),
		rotatelogs.WithRotationTime(time.Duration(rotationTime)*time.Minute),
	)
}

func doFlush(writer *bufio.Writer, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	writer.Flush()
}

func doWrite(writer *bufio.Writer, buf []byte, mu *sync.Mutex) (int, error) {
	mu.Lock()
	defer mu.Unlock()
	if writer.Available() > 0 && len(buf) > writer.Available() {
		writer.Flush()
	}
	return writer.Write(buf)
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

	iow, err := writeTo(opts.LogDir, opts.RotationTime, opts.MaxAge)
	if err != nil {
		log.Fatalf("failed initialize logger: %v", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	mu := new(sync.Mutex)
	writer := bufio.NewWriterSize(iow, 1024*1024)
	defer doFlush(writer, mu)
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			doFlush(writer, mu)
		}
	}()

	bufioChan := make(chan error, 1)

	stdin := bufio.NewScanner(os.Stdin)
	stdin.Buffer(make([]byte, 10000), 1000000)
	go func() {
		for stdin.Scan() {
			buf := make([]byte, 0, 2000)
			if opts.WithTime {
				buf = append(buf, '[')
				buf = append(buf, currentTime()...)
				buf = append(buf, ']', ' ')
			}
			buf = append(buf, stdin.Bytes()...)
			buf = append(buf, '\n')
			_, err = doWrite(writer, buf, mu)
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
