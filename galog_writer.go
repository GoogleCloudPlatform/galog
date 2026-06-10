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
	"context"
	"fmt"
	"io"
)

// writerBackend is a shared implementation for writer-based backends.
type writerBackend struct {
	// backendID is the internal ID of the backend.
	backendID string
	// config is a pointer to the generic Config interface implementation.
	config *backendConfig
	// writer is the target writer (e.g., os.Stdout, os.Stderr, or a buffer).
	writer io.Writer
	// skip is a function determining if a log entry level should be skipped.
	skip func(Level) bool
	// sync is a function to flush/sync the backend storage.
	sync func() error
}

// ID returns the backend's ID.
func (wb *writerBackend) ID() string {
	return wb.backendID
}

// Config returns the backend's configuration.
func (wb *writerBackend) Config() Config {
	return wb.config
}

// Shutdown flushes/syncs the backend.
func (wb *writerBackend) Shutdown(context.Context) error {
	if wb.sync == nil {
		return nil
	}
	if err := wb.sync(); err != nil {
		return fmt.Errorf("failed to flush backend %s: %w", wb.backendID, err)
	}
	return nil
}

// Log formats and writes the entry to the output stream.
func (wb *writerBackend) Log(entry *LogEntry) error {
	if wb.skip != nil && wb.skip(entry.Level) {
		return nil
	}

	format := wb.config.Format(entry.Level)

	message, err := entry.Format(format + "\n")
	if err != nil {
		return fmt.Errorf("failed to format log level: %w", err)
	}

	n, err := wb.writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write log to backend %s: %w", wb.backendID, err)
	}

	if n != len(message) {
		return fmt.Errorf("failed to write message, wrote %d bytes out of %d bytes", n, len(message))
	}

	return nil
}
