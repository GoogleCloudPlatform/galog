//  Copyright 2024 Google LLC
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package galog

import (
	"fmt"
	"os"
)

const (
	// defaultFileQueueSize defines the default queue size of the file backend
	// implementation. In general writing to a file should not be an expensive
	// operation and would complete pretty quickly, however in situations such as
	// when the storage is full (even temporarily) we'll need some
	// queueing/caching we don't want it to grow indefinitely but to have a
	// reasonable size.
	defaultFileQueueSize = 1000
)

// FileBackend is an implementation for logging to a file.
type FileBackend struct {
	// backendID is the internal id of this file backend.
	backendID string
	// logFilePath is the path of the log file.
	logFilePath string
	// config is a pointer to the generic Config interface implementation.
	config *backendConfig
}

// NewFileBackend returns a Backend implementation that will log out to
// the file specified by logFilePath.
func NewFileBackend(logFilePath string) *FileBackend {
	res := &FileBackend{
		backendID:   "log-backend,file",
		logFilePath: logFilePath,
		config:      newBackendConfig(defaultFileQueueSize),
	}

	res.config.SetFormat(ErrorLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: {{.Message}}`)
	res.config.SetFormat(DebugLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}`)

	return res
}

// ID returns the file backend implementation's ID.
func (fb *FileBackend) ID() string {
	return fb.backendID
}

// Log prints the log entry to setup file.
func (fb *FileBackend) Log(entry *LogEntry) error {
	logFile, err := os.OpenFile(fb.logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create/open log file: %s", fb.logFilePath)
	}
	defer logFile.Close()

	format := fb.config.Format(entry.Level)
	message, err := entry.Format(format + "\n")
	if err != nil {
		return fmt.Errorf("failed to format log message: %+v", err)
	}

	n, err := logFile.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write log to file: %+v", err)
	}

	if n != len(message) {
		return fmt.Errorf("failed to write the message, wrote %d bytes out of %d bytes", n, len(message))
	}

	return nil
}

// Config returns the backend configuration of the file backend.
func (fb *FileBackend) Config() Config {
	return fb.config
}

// Flush is a no-op implementation for file backend as we are opening the file
// for every log operation.
func (fb *FileBackend) Flush() error {
	return nil
}
