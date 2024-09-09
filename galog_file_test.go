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
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
)

func TestFileInvalidFormat(t *testing.T) {
	logFile := path.Join(t.TempDir(), "galogtest.log")
	be := NewFileBackend(logFile)

	be.Config().SetFormat(ErrorLevel, "{{.Foobar}}")

	entry := newEntry(ErrorLevel, "", "foobar")
	err := be.Log(entry)
	if err == nil {
		t.Fatalf("Log() expected error, got nil")
	}
}

func TestFileSuccess(t *testing.T) {
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
			desc:    "warning_level",
			message: "foo bar",
			level:   WarningLevel,
			want:    "[WARNING]: foo bar\n",
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logFile := path.Join(t.TempDir(), fmt.Sprintf("galogtest-%d.log", i))
			be := NewFileBackend(logFile)

			if be.Config() == nil {
				t.Fatal("NewStderrBackend() failed: Config() returned nil")
			}

			if be.ID() == "" {
				t.Fatal("NewStderrBackend() failed: ID() returned empty string")
			}

			entry := newEntry(tc.level, "", tc.message)
			err := be.Log(entry)
			if err != nil {
				t.Fatalf("Log() failed: %v", err)
			}

			fileContent, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) failed: %v", logFile, err)
			}

			if !strings.HasSuffix(string(fileContent), tc.want) {
				t.Fatalf("Log() got: %s, want suffix: %s", string(fileContent), tc.want)
			}
		})
	}
}
