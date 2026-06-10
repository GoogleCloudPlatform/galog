//  Copyright 2026 Google LLC
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
	"os"
)

const (
	// defaultStdoutQueueSize defines the default queue size of the stdout
	// backend implementation. In general writing to stdout doesn't require
	// caching or queueing, we set a limit to avoid the queue to grow
	// indefinitely in case of any disastrous behavior of the OS - as 0 means
	// "grow indefinitely".
	defaultStdoutQueueSize = 10
)

// StdoutBackend is a simple backend implementation for logging to stdout.
type StdoutBackend struct {
	*writerBackend
}

// NewStdoutBackend returns a Backend implementation that will log out to the
// process' stdout.
func NewStdoutBackend() *StdoutBackend {
	res := &StdoutBackend{
		writerBackend: &writerBackend{
			backendID: "log-backend,stdout",
			config:    newBackendConfig(defaultStdoutQueueSize),
			writer:    os.Stdout,
			skip:      func(lvl Level) bool { return lvl == ErrorLevel || lvl == FatalLevel },
			sync:      os.Stdout.Sync,
		},
	}

	res.config.SetFormat(InfoLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: {{.Message}}`)
	res.config.SetFormat(DebugLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}`)

	return res
}
