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
	"context"
	"log/syslog"
	"strings"
	"testing"
	"time"
)

func TestSyslog(t *testing.T) {
	if _, err := syslog.New(syslog.LOG_DAEMON|syslog.LOG_INFO, "test"); err != nil {
		t.Skipf("syslog not found, skipping test: %v", err)
	}

	tl := newTestLogger()
	tl.SetQueueRetryFrequency(time.Millisecond)
	tl.SetLevel(DebugLevel)
	be := NewSyslogBackend("guest-agent")
	be.metrics = new(syslogMetrics)

	tl.RegisterBackend(context.Background(), be)

	tests := []struct {
		desc string
		fn   func(args ...any)
	}{
		{
			desc: "debug",
			fn:   tl.Debug,
		},
		{
			desc: "info",
			fn:   tl.Info,
		},
		{
			desc: "warn",
			fn:   tl.Warn,
		},
		{
			desc: "error",
			fn:   tl.Error,
		},
	}

	for writtenEntries, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tc.fn("foobar")
			success := false
			// retry for 10 milliseconds
			for i := 0; i < 10; i++ {
				be.metrics.mu.Lock()
				ctr := be.metrics.success
				be.metrics.mu.Unlock()

				success = ctr == int64(writtenEntries+1)
				if success {
					break
				}

				time.Sleep(time.Millisecond)
			}

			if be.metrics.errors != 0 {
				t.Errorf("got errors %d, want 0. Errors: \n%s\n", be.metrics.errors, strings.Join(be.metrics.errorMsgs, "\n"))
			}

			if !success {
				t.Errorf("got success %d, want %d", be.metrics.success, writtenEntries+1)
			}
		})
	}
}
