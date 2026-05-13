package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

type Logger struct {
	file *os.File
	log  *log.Logger
}

func New(logFile string) (*Logger, error) {
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	var file *os.File
	if logFile != "" {
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		file = f
		writers = append(writers, f)
	}

	return &Logger{
		file: file,
		log:  log.New(io.MultiWriter(writers...), "", log.LstdFlags),
	}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) Infof(format string, args ...any) {
	l.log.Printf("[INFO] "+format, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log.Printf("[WARN] "+format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log.Printf("[ERROR] "+format, args...)
}
