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
	"sync"
)

const (
	// defaultSyslogQueueSize defines the default queue size of the syslog
	// backend implementation. In general writing to syslog is not an expensive
	// operation and completes quickly. Default to a reasonable queue size to
	// prevent issues with full disks or any syslog misbehaving.
	//
	// The size was picked based on a reasonable in memory stored cache
	// considering an average of 100 characters per log message (so 100
	// bytes per message) * 1024 entries equates to ~100kb of log entries.
	defaultSyslogQueueSize = 1024

	// syslogBackendID is the internal id of this syslog backend.
	syslogBackendID = "log-backend,syslog"
)

// syslogMetrics is a struct used to collect syslog metrics.
type syslogMetrics struct {
	// mu protects metric updates.
	mu sync.Mutex
	// success is the number of successful log entry writes.
	success int64
	// errors is the number of failed log entry writes.
	errors int64
	// errorMsgs is the list of error messages.
	errorMsgs []string
}

// SyslogBackend is an implementation for logging to the linux syslog.
type SyslogBackend struct {
	// backendID is the internal id of this syslog backend.
	backendID string
	// ident is the syslog entry ident, it's passed down to the syslog writer.
	ident string
	// config is the generic Config interface implementation.
	config *backendConfig
	// metrics is the syslog metrics.
	metrics *syslogMetrics
}

// ID returns the syslog backend implementation's ID.
func (sb *SyslogBackend) ID() string {
	return sb.backendID
}

// Flush is a no-op implementation for syslog backend as we are opening
// (and closing) syslogger for every log operation.
func (sb *SyslogBackend) Flush() error {
	return nil
}

// Config returns the configuration of the syslog backend.
func (sb *SyslogBackend) Config() Config {
	return sb.config
}
