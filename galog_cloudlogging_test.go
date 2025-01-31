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
	"errors"
	"testing"
	"time"
)

func TestCloudLogging(t *testing.T) {
	opts := CloudOptions{
		FlushCadence:              time.Second,
		Project:                   "test-project",
		WithoutAuthentication:     true,
		Instance:                  "test-instance",
		UserAgent:                 "galog Agent",
		DisableClientErrorLogging: true,
	}

	ctx := context.Background()
	be, err := NewCloudBackend(ctx, CloudLoggingInitModeActive, &opts)
	if err != nil {
		t.Fatalf("NewCloudBackend() failed: %v", err)
	}

	if err := be.InitClient(ctx, &opts); !errors.Is(err, errCloudLoggingAlreadyInitialized) {
		t.Fatalf("InitClient() = %v, want: %v", err, errCloudLoggingAlreadyInitialized)
	}

	if be.ID() == "" {
		t.Fatalf("ID() == \"\", want: non-empty")
	}

	if be.Config() == nil {
		t.Fatalf("Config() == nil, want: non-nil")
	}

	if be.periodicLogger.interval != DefaultClientErrorInterval {
		t.Fatalf("periodicLogger.interval = %v, want: %v", be.periodicLogger.interval, DefaultClientErrorInterval)
	}

	if !be.disableClientErrorLogging {
		t.Fatalf("disableClientErrorLogging = false, want: true")
	}

	err = be.Log(&LogEntry{When: time.Now(), Message: "foobar"})
	if err != nil {
		t.Fatalf("Log() failed: %v", err)
	}
}

func TestCloudInvalidFormat(t *testing.T) {
	opts := CloudOptions{
		FlushCadence:          time.Second,
		Project:               "test-project",
		WithoutAuthentication: true,
		Instance:              "test-instance",
		UserAgent:             "galog Agent",
	}

	ctx := context.Background()
	be, err := NewCloudBackend(ctx, CloudLoggingInitModeActive, &opts)
	if err != nil {
		t.Fatalf("NewCloudBackend() failed: %v", err)
	}

	if err := be.InitClient(ctx, &opts); !errors.Is(err, errCloudLoggingAlreadyInitialized) {
		t.Fatalf("InitClient() = %v, want: %v", err, errCloudLoggingAlreadyInitialized)
	}

	be.Config().SetFormat(ErrorLevel, "{{.InvalidField}}")

	err = be.Log(&LogEntry{When: time.Now(), Message: "foobar"})
	if err == nil {
		t.Fatalf("Log() = nil, want: non-nil")
	}
}

func TestCloudLoggingLazyInit(t *testing.T) {
	be, err := NewCloudBackend(context.Background(), CloudLoggingInitModeLazy, &CloudOptions{})
	if err != nil {
		t.Fatalf("NewCloudBackend() failed: %v", err)
	}

	err = be.Log(&LogEntry{When: time.Now(), Message: "foobar"})
	if !errors.Is(err, errCloudLoggingNotInitialized) {
		t.Fatalf("Log() = %v, want: %v", err, errCloudLoggingNotInitialized)
	}

	if err := be.Shutdown(context.Background()); !errors.Is(err, errCloudLoggingNotInitialized) {
		t.Fatalf("Shutdown(ctx) = %v, want: %v", err, errCloudLoggingNotInitialized)
	}
}

func TestPeriodicLogger(t *testing.T) {

	tests := []struct {
		name              string
		wantLog           bool
		lastlog           time.Time
		firstRunHasPassed bool
		interval          time.Duration
	}{
		{
			name:     "first_log_ignore_lastlog",
			wantLog:  true,
			lastlog:  time.Now().Add(-time.Second),
			interval: time.Second * 2,
		},
		{
			name:              "next_log_interval_not_passed",
			wantLog:           false,
			firstRunHasPassed: true,
			lastlog:           time.Now().Add(-time.Second * 1),
			interval:          time.Second * 2,
		},
		{
			name:              "next_log_interval_passed",
			firstRunHasPassed: true,
			wantLog:           true,
			interval:          time.Second * 2,
			lastlog:           time.Now().Add(-time.Second * 3),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &periodicLogger{
				interval:       tc.interval,
				lastLog:        tc.lastlog,
				firstRunPassed: tc.firstRunHasPassed,
			}
			gotLog := logger.log(errors.New("test error"))
			if gotLog != tc.wantLog {
				t.Errorf("periodicLogger.log() logged = %t, want: %t", gotLog, tc.wantLog)
			}

			if !tc.wantLog {
				return
			}

			if logger.lastLog.Equal(tc.lastlog) {
				t.Error("periodicLogger.log() did not update lastLog")
			}
			if !logger.firstRunPassed {
				t.Error("periodicLogger.log() did not set firstRunPassed")
			}
		})
	}
}
