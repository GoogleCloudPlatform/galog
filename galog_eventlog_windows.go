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

//go:build windows

package galog

import (
	"fmt"
	"strings"

	"golang.org/go/x/sys/windows/svc/eventlog"
)

// NewEventlogBackend returns a new EventlogBackend implementation.
func NewEventlogBackend(eventID uint32, ident string) (*EventlogBackend, error) {
	res := &EventlogBackend{
		backendID: eventlogBackendID,
		eventID:   eventID,
		ident:     ident,
		config:    newBackendConfig(defaultEventlogQueueSize),
	}

	res.config.SetFormat(ErrorLevel, "{{if .Prefix}}{{.Prefix}}: {{end}}{{.Message}}")
	res.config.SetFormat(DebugLevel, "{{if .Prefix}}{{.Prefix}}: {{end}}({{.File}}:{{.Line}}) {{.Message}}")

	return res, nil
}

// Log prints the log entry to eventlog.
func (eb *EventlogBackend) Log(entry *LogEntry) error {
	if err := eb.writeEntry(entry); err != nil {
		eb.recordMetric(false)
		return err
	}
	eb.recordMetric(true)
	return nil
}

func (eb *EventlogBackend) writeEntry(entry *LogEntry) error {
	format := eb.config.Format(entry.Level)
	logMessage, err := entry.Format(format)
	if err != nil {
		return fmt.Errorf("failed to format event log message: %+v", err)
	}

	// Only attempt to install the event logger if we've not managed to register
	// before.
	if !eb.registered {
		err := eventlog.InstallAsEventCreate(eb.ident, eventlog.Info|eventlog.Warning|eventlog.Error)
		if err != nil && !strings.Contains(err.Error(), "registry key already exists") {
			return fmt.Errorf("failed to install eventlog: %+v", err)
		}
		eb.registered = true
	}

	writer, err := eventlog.Open(eb.ident)
	if err != nil {
		return fmt.Errorf("failed to open eventlog: %+v", err)
	}
	defer writer.Close()

	ops := map[Level]func(uint32, string) error{
		DebugLevel:   writer.Info,
		InfoLevel:    writer.Info,
		WarningLevel: writer.Warning,
		ErrorLevel:   writer.Error,
		FatalLevel:   writer.Error,
	}

	fn, found := ops[entry.Level]
	if !found {
		return fmt.Errorf("unsupported event log level: %+v", entry.Level)
	}

	if err := fn(eb.eventID, logMessage); err != nil {
		return fmt.Errorf("writing to eventlog: %+v", err)
	}

	return nil
}

// recordMetric records number of success or failures of eventlog writes.
func (eb *EventlogBackend) recordMetric(success bool) {
	if eb.metrics == nil {
		return
	}
	if success {
		eb.metrics.success++
	} else {
		eb.metrics.errors++
	}
}
