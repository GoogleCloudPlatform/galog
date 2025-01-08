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
	"context"
	"fmt"
	"io"
	"os"
)

const (
	// defaultStderrQueueSize defines the default queue size of the stderr backend
	// implementation. In general writing to stderr doesn't require caching or
	// queueing, we are set a limit to avoid the queue to grow indefinitely in
	// case of any disastrous behavior of the OS - as 0 means "grow indefinitely".
	defaultStderrQueueSize = 10
)

// StderrBackend is a simple backend implementation for logging to stderr.
type StderrBackend struct {
	// backendID is the internal id of this backend.
	backendID string
	// config is a pointer to the generic Config interface implementation.
	config *backendConfig
	// writer by default it's set to use os.Stderr, tests might override it to a
	// local writer.
	writer io.Writer
}

// NewStderrBackend returns a Backend implementation that will log out to
// the process' stderr.
func NewStderrBackend(writer io.Writer) *StderrBackend {
	res := &StderrBackend{
		backendID: "log-backend,stderr",
		config:    newBackendConfig(defaultStderrQueueSize),
		writer:    writer,
	}

	res.config.SetFormat(ErrorLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: {{.Message}}`)
	res.config.SetFormat(DebugLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}`)

	return res
}

// ID returns the stderr backend implementation's ID.
func (wb *StderrBackend) ID() string {
	return wb.backendID
}

// Log prints the log entry to stderr.
func (wb *StderrBackend) Log(entry *LogEntry) error {
	format := wb.config.Format(entry.Level)

	message, err := entry.Format(format + "\n")
	if err != nil {
		return fmt.Errorf("failed to format log level: %+v", err)
	}

	n, err := wb.writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write log to stderr: %+v", err)
	}

	if n != len(message) {
		return fmt.Errorf("failed to write the message, wrote %d bytes out of %d bytes", n, len(message))
	}

	return nil
}

// Config returns the backend configuration of the stderr backend.
func (wb *StderrBackend) Config() Config {
	return wb.config
}

// Flush flushes the stderr file.
func (wb *StderrBackend) Flush(context.Context) error {
	if err := os.Stderr.Sync(); err != nil {
		return fmt.Errorf("failed to flush stderr: %+v", err)
	}
	return nil
}
