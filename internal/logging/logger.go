package logging

import (
	"log"
	"os"
)

var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func Init() {
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func Info(v ...any) {
	if InfoLogger != nil {
		InfoLogger.Println(v...)
	}
}

func Error(v ...any) {
	if ErrorLogger != nil {
		ErrorLogger.Println(v...)
	}
}
