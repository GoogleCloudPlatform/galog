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
	"bytes"
	"strings"
	"testing"
	"time"
)

// helper1 and newTestEntry together wrapper functions to simulate a stack
// depth of 3 layers between the test code calling newTestEntry and the
// runtime.Caller(3) call site inside newEntry.
func helper1(level Level, prefix string, msg string) *LogEntry {
	return newEntry(level, prefix, msg)
}

func newTestEntry(level Level, prefix string, msg string) *LogEntry {
	return helper1(level, prefix, msg)
}

func TestStdoutWriteFailure(t *testing.T) {
	logBuffer := &errorWriter{failureType: writeFailure}
	be := NewStdoutBackend()
	be.writer = logBuffer

	entry := newTestEntry(InfoLevel, "", "foobar")
	if err := be.Log(entry); err == nil {
		t.Fatalf("Log(%+v) succeeded, want error due to write failure", entry)
	}
}

func TestStdoutWriteLenFailure(t *testing.T) {
	logBuffer := &errorWriter{failureType: writeLenFailure}
	be := NewStdoutBackend()
	be.writer = logBuffer

	entry := newTestEntry(InfoLevel, "", "foobar")
	if err := be.Log(entry); err == nil {
		t.Fatalf("Log(%+v) succeeded, want error due to write len failure", entry)
	}
}

func TestStdoutInvalidFormat(t *testing.T) {
	logBuffer := bytes.NewBuffer(nil)
	be := NewStdoutBackend()
	be.writer = logBuffer

	be.Config().SetFormat(InfoLevel, "{{.Foobar}}")

	entry := newTestEntry(InfoLevel, "", "foobar")
	if err := be.Log(entry); err == nil {
		t.Fatalf("Log(%+v) succeeded, want error due to invalid format", entry)
	}
}

func TestStdoutSuccess(t *testing.T) {
	tests := []struct {
		desc    string
		message string
		level   Level
		prefix  string
		want    string
	}{
		{
			desc:    "error_level_skip",
			message: "foo bar",
			level:   ErrorLevel,
			want:    "",
		},
		{
			desc:    "fatal_level_skip",
			message: "foo bar",
			level:   FatalLevel,
			want:    "",
		},
		{
			desc:    "warning_level",
			message: "foo bar",
			level:   WarningLevel,
			want:    "[WARNING]: foo bar\n",
		},
		{
			desc:    "info_level",
			message: "foo bar",
			level:   InfoLevel,
			want:    "[INFO]: foo bar\n",
		},
		{
			desc:    "info_level_with_prefix",
			message: "foo bar",
			level:   InfoLevel,
			prefix:  "my-prefix",
			want:    " my-prefix: [INFO]: foo bar\n",
		},
		{
			desc:    "debug_level",
			message: "foo bar",
			level:   DebugLevel,
			want:    "[DEBUG]:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logBuffer := bytes.NewBuffer(nil)
			be := NewStdoutBackend()
			be.writer = logBuffer
			if be.Config() == nil {
				t.Fatal("NewStdoutBackend() failed: Config() returned nil")
			}

			entry := newTestEntry(tc.level, tc.prefix, tc.message)
			if err := be.Log(entry); err != nil {
				t.Fatalf("Log(%+v) failed: %v", entry, err)
			}

			got := logBuffer.String()

			if tc.want == "" {
				if logBuffer.Len() != 0 {
					t.Fatalf("Log(%+v) output = %q, want empty", entry, got)
				}
				return
			}

			if tc.level == DebugLevel {
				if !strings.Contains(got, "[DEBUG]:") || !strings.Contains(got, "galog_stdout_test.go:") || !strings.HasSuffix(got, tc.message+"\n") {
					t.Fatalf("Log(%+v) output = %q, want [DEBUG]:, galog_stdout_test.go:, and suffix %q", entry, got, tc.message+"\n")
				}
			} else {
				if !strings.HasSuffix(got, tc.want) {
					t.Fatalf("Log(%+v) output = %q, want suffix %q", entry, got, tc.want)
				}
			}
			Shutdown(time.Millisecond)
		})
	}
}
