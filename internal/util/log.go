package util

import (
	"fmt"
	"log"
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
}

type colorLogger struct {
	label string
}

func NewColorLogger(label string) Logger {
	return &colorLogger{label: label}
}

func (l *colorLogger) logf(level, format string, args ...interface{}) {
	// Customize color based on log level
	colorCode := ""
	switch level {
	case "debug", "info":
		colorCode = "\033[36m" // Light blue
	case "warning":
		colorCode = "\033[33m" // Yellow
	case "error":
		colorCode = "\033[31m" // Red
	case "panic":
		colorCode = "\033[31m" // Bold red for panic
	}
	// Print message with label and color
	labelToUse := l.label
	if len(labelToUse) < 6 {
		labelToUse = fmt.Sprintf("%-6s", labelToUse)
	}
	log.Printf(" %s %s %s\n", labelToUse, colorCode, fmt.Sprintf(format, args...))
	// Reset color code after printing
	fmt.Print("\033[0m")
}

func (l *colorLogger) Debugf(format string, args ...interface{}) {
	l.logf("debug", format, args...)
}

func (l *colorLogger) Infof(format string, args ...interface{}) {
	l.logf("info", format, args...)
}

func (l *colorLogger) Warningf(format string, args ...interface{}) {
	l.logf("warning", format, args...)
}

func (l *colorLogger) Errorf(format string, args ...interface{}) {
	l.logf("error", format, args...)
}

func (l *colorLogger) Panicf(format string, args ...interface{}) {
	l.logf("panic", format, args...)
	panic(fmt.Sprintf(format, args...))
}

var Log Logger = NewColorLogger("Kiruna")
