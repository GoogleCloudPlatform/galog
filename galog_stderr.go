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
	"os"
)

const (
	// defaultStderrQueueSize defines the default queue size of the stderr
	// backend implementation. In general writing to stderr doesn't require
	// caching or queueing, we set a limit to avoid the queue to grow
	// indefinitely in case of any disastrous behavior of the OS - as 0 means
	// "grow indefinitely".
	defaultStderrQueueSize = 10
)

// StderrBackend is a simple backend implementation for logging to stderr.
type StderrBackend struct {
	*writerBackend
}

// NewStderrBackend returns a Backend implementation that will log out to
// the process' stderr. It always writes to os.Stderr.
func NewStderrBackend() *StderrBackend {
	res := &StderrBackend{
		writerBackend: &writerBackend{
			backendID: "log-backend,stderr",
			config:    newBackendConfig(defaultStderrQueueSize),
			writer:    os.Stderr,
			skip:      func(lvl Level) bool { return lvl != ErrorLevel && lvl != FatalLevel },
			sync:      os.Stderr.Sync,
		},
	}

	res.config.SetFormat(ErrorLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: {{.Message}}`)
	res.config.SetFormat(DebugLevel,
		`{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} {{if .Prefix}} {{.Prefix}}: {{end}}[{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}`)

	return res
}
