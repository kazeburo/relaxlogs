package logger

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

const timeFormat = "02/Jan/2006:15:04:05 +0900"

// RelaxLogger bufio with lock
type RelaxLogger struct {
	sync.Mutex
	w         *bufio.Writer
	withTime  bool
	timestamp []byte
	unix      int64
}

func makeLogger(logDir string, rotationTime int64, maxAge int64) (io.Writer, error) {
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

func NewRelaxLogger(withTime bool, bufferSize int, logDir string, rotationTime int64, maxAge int64) (*RelaxLogger, error) {
	logger, err := makeLogger(logDir, rotationTime, maxAge)
	if err != nil {
		return nil, err
	}

	bufsize := bufferSize + 1
	if withTime {
		bufsize += len(timeFormat) + 3
	}

	rl := &RelaxLogger{
		w:        bufio.NewWriterSize(logger, bufsize),
		withTime: withTime,
	}
	return rl, nil
}

// Flush with lock
func (rl *RelaxLogger) Flush() {
	rl.Lock()
	defer rl.Unlock()
	rl.w.Flush()
}

// Write with lock
func (rl *RelaxLogger) Write(buf []byte) (int, error) {
	rl.Lock()
	defer rl.Unlock()
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

func (rl *RelaxLogger) getTimestamp() []byte {
	now := time.Now().Unix()
	if now == rl.unix {
		return rl.timestamp
	}
	rl.unix = now
	rl.timestamp = []byte("[" + time.Unix(now, 0).Format(timeFormat) + "] ")
	return rl.timestamp
}
