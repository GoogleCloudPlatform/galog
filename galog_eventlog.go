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
)

var (
	// defaultEventlogQueueSize defines the default queue size of the windows
	// event log backend implementation. In general writing to event log should
	// not be an expensive operation and would complete pretty quickly. We are
	// defining a reasonable  queue size to prevent issues with full disks or any
	// event log misbehaving.
	defaultEventlogQueueSize = 100

	// eventlogBackendID is the internal id of this eventlog backend.
	eventlogBackendID = "log-backend,eventlog"
)

// eventlogMetrics is a struct used to collect eventlog metrics.
type eventlogMetrics struct {
	// success is the number of successful log entry writes.
	success int64
	// errors is the number of failed log entry writes.
	errors int64
}

// EventlogBackend implements the Backend interface for logging to windows
// eventlog.
type EventlogBackend struct {
	// backendID of the backend implementation.
	backendID string
	// ID of the event to log to.
	eventID uint32
	// ident is the service's ident registered with eventlog.
	ident string
	// config is the configuration of the backend.
	config *backendConfig
	// metrics is the eventlog metrics.
	metrics *eventlogMetrics
	// registered is true if the backend is registered with eventlog.
	registered bool
}

// ID returns the eventlog backend implementation's ID.
func (eb *EventlogBackend) ID() string {
	return eb.backendID
}

// Config returns the configuration of the eventlog backend.
func (eb *EventlogBackend) Config() Config {
	return eb.config
}

// Shutdown is a no-op implementation for event backend as we are opening eventlog
// for every log operation.
func (eb *EventlogBackend) Shutdown(context.Context) error {
	return nil
}
