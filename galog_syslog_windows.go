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

//go:build !linux

package galog

// NewSyslogBackend is a no-op backend for windows as there's no OS's native
// syslog framework available for windows.
func NewSyslogBackend(ident string) *SyslogBackend {
	return &SyslogBackend{
		backendID: syslogBackendID,
		ident:     ident,
		config:    newBackendConfig(defaultSyslogQueueSize),
	}
}

// Log is no-op for syslog windows implementation as there's no OS's native
// syslog framework available for windows.
func (sb *SyslogBackend) Log(entry *LogEntry) error {
	return nil
}
