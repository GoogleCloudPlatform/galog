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

//go:build linux

package galog

import (
	"fmt"
	"log/syslog"
)

// NewSyslogBackend returns a Backend implementation that will log out to
// the underlying system's syslog framework.
func NewSyslogBackend(ident string) *SyslogBackend {
	res := &SyslogBackend{
		backendID: syslogBackendID,
		ident:     ident,
		config:    newBackendConfig(defaultSyslogQueueSize),
	}

	res.config.SetFormat(ErrorLevel, "{{if .Prefix}}{{.Prefix}}: {{end}}{{.Message}}")
	res.config.SetFormat(DebugLevel, "{{if .Prefix}}{{.Prefix}}: {{end}}({{.File}}:{{.Line}}) {{.Message}}")

	return res
}

// recordMetric records number of success or failures of syslog writes.
func (sb *SyslogBackend) recordMetric(err error) {
	if sb.metrics == nil {
		return
	}
	sb.metrics.mu.Lock()
	defer sb.metrics.mu.Unlock()
	if err == nil {
		sb.metrics.success++
	} else {
		sb.metrics.errors++
		sb.metrics.errorMsgs = append(sb.metrics.errorMsgs, err.Error())
	}
}

// Log prints the log entry to syslog.
func (sb *SyslogBackend) Log(entry *LogEntry) error {
	if err := sb.writeEntry(entry); err != nil {
		sb.recordMetric(err)
		return err
	}
	sb.recordMetric(nil)
	return nil
}

func (sb *SyslogBackend) writeEntry(entry *LogEntry) error {
	writer, err := syslog.New(syslog.LOG_DAEMON|syslog.LOG_INFO, sb.ident)
	if err != nil {
		return fmt.Errorf("opening syslog: %v", err)
	}
	defer writer.Close()

	format := sb.config.Format(entry.Level)
	message, err := entry.Format(format)
	if err != nil {
		return fmt.Errorf("formating syslog message: %v", err)
	}

	ops := map[Level]func(string) error{
		DebugLevel:   writer.Debug,
		InfoLevel:    writer.Info,
		WarningLevel: writer.Warning,
		ErrorLevel:   writer.Err,
		FatalLevel:   writer.Crit,
	}

	if ptr, found := ops[entry.Level]; found {
		if err := ptr(message); err != nil {
			return fmt.Errorf("writing to syslog: %v", err)
		}
	}

	return nil
}
