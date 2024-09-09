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

	"go.bug.st/serial"
)

const (
	// DefaultSerialBaud is the default serial baud for serial port writing.
	DefaultSerialBaud = 115200
	// defaultSerialQueueSize is the default queue size of the serial backend.
	defaultSerialQueueSize = 1000
)

// SerialBackend is an implementation for logging to serial.
type SerialBackend struct {
	// backendID is the serial backend ID.
	backendID string
	// opts is the serial configuration options.
	opts *SerialOptions
	// config is a pointer to the generic Config interface implementation.
	config *backendConfig
}

// SerialOptions contains the options for serial backend.
type SerialOptions struct {
	// Port is the serial port name to be written to.
	Port string
	// Baud is the serial port baud.
	Baud int
}

// NewSerialBackend returns a Backend implementation that will log out to
// the configured serial port.
func NewSerialBackend(ctx context.Context, opts *SerialOptions) *SerialBackend {
	res := &SerialBackend{
		backendID: "log-backend,serial",
		opts:      opts,
		config:    newBackendConfig(defaultStderrQueueSize),
	}

	res.config.SetFormat(ErrorLevel,
		`{{if .Prefix}}{{.Prefix}}: {{end}}{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} [{{.Level}}]: {{.Message}}`)
	res.config.SetFormat(DebugLevel,
		`{{if .Prefix}}{{.Prefix}}: {{end}}{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} [{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}`)

	return res
}

// Log prints the log entry to setup serial.
func (sb *SerialBackend) Log(entry *LogEntry) error {
	config := &serial.Mode{
		BaudRate: sb.opts.Baud,
	}

	port, err := serial.Open(sb.opts.Port, config)
	if err != nil {
		return fmt.Errorf("error opening serial port: %+v", err)
	}
	defer port.Close()

	format := sb.config.Format(entry.Level)
	message, err := entry.Format(format + "\n")
	if err != nil {
		return fmt.Errorf("failed to format log level: %+v", err)
	}

	nn, err := port.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write log to serial: %+v", err)
	}

	if nn != len(message) {
		return fmt.Errorf("failed to write the message, wrote %d bytes out of %d bytes", nn, len(message))
	}

	return nil
}

// ID returns the cloud logging backend implementation's ID.
func (sb *SerialBackend) ID() string {
	return sb.backendID
}

// Flush is a no-op implementation for serial backend as we are opening the file
// for every log operation.
func (sb *SerialBackend) Flush() error {
	return nil
}

// Config returns the configuration of the serial backend.
func (sb *SerialBackend) Config() Config {
	return sb.config
}
