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
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	writeFailure int = iota
	writeLenFailure
)

type errorWriter struct {
	failureType int
}

func (ew errorWriter) Write(data []byte) (int, error) {
	if ew.failureType == writeFailure {
		return 0, fmt.Errorf("injected write error")
	} else if ew.failureType == writeLenFailure {
		return 0, nil
	}
	return len(data), nil
}

func TestStderrWriteFailure(t *testing.T) {
	logBuffer := &errorWriter{failureType: writeFailure}
	be := NewStderrBackend(logBuffer)

	entry := newEntry(ErrorLevel, "", "foobar")
	err := be.Log(entry)
	if err == nil {
		t.Fatalf("Log() expected error, got nil")
	}
}

func TestStderrWriteLenFailure(t *testing.T) {
	logBuffer := &errorWriter{failureType: writeLenFailure}
	be := NewStderrBackend(logBuffer)

	entry := newEntry(ErrorLevel, "", "foobar")
	err := be.Log(entry)
	if err == nil {
		t.Fatalf("Log() expected error, got nil")
	}
}

func TestStderrInvalidFormat(t *testing.T) {
	logBuffer := bytes.NewBuffer(nil)
	be := NewStderrBackend(logBuffer)

	be.Config().SetFormat(ErrorLevel, "{{.Foobar}}")

	entry := newEntry(ErrorLevel, "", "foobar")
	err := be.Log(entry)
	if err == nil {
		t.Fatalf("Log() expected error, got nil")
	}
}

func TestStderrSuccess(t *testing.T) {
	tests := []struct {
		desc    string
		message string
		level   Level
		want    string
	}{
		{
			desc:    "error_level",
			message: "foo bar",
			level:   ErrorLevel,
			want:    "[ERROR]: foo bar\n",
		},
		{
			desc:    "warning_level_skip",
			message: "foo bar",
			level:   WarningLevel,
			want:    "",
		},
		{
			desc:    "info_level_skip",
			message: "foo bar",
			level:   InfoLevel,
			want:    "",
		},
		{
			desc:    "debug_level_skip",
			message: "foo bar",
			level:   DebugLevel,
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logBuffer := bytes.NewBuffer(nil)
			be := NewStderrBackend(logBuffer)
			if be.Config() == nil {
				t.Fatal("NewStderrBackend() failed: Config() returned nil")
			}

			entry := newEntry(tc.level, "", tc.message)
			if err := be.Log(entry); err != nil {
				t.Fatalf("Log() failed: %v", err)
			}

			if tc.want == "" && logBuffer.Len() != 0 {
				t.Fatalf("Log() got: %s, want empty", logBuffer.String())
			}

			if !strings.HasSuffix(logBuffer.String(), tc.want) {
				t.Fatalf("Log() got: %s, want suffix: %s", logBuffer.String(), tc.want)
			}
			Shutdown(time.Millisecond)
		})
	}
}
