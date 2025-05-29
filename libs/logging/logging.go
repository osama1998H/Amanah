package logging

import (
	"io"
	"log"
	"os"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

// SetOutput sets the destination for the logger.
func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

// Info logs informational messages.
func Info(format string, v ...interface{}) {
	logger.Printf("[INFO] "+format, v...)
}

// Error logs error messages.
func Error(format string, v ...interface{}) {
	logger.Printf("[ERROR] "+format, v...)
}
