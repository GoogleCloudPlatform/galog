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
	"context"
	"testing"
	"time"
)

func TestEventlog(t *testing.T) {
	tests := []struct {
		desc string
		fn   func(args ...any)
	}{
		{
			desc: "debug",
			fn:   Debug,
		},
		{
			desc: "info",
			fn:   Info,
		},
		{
			desc: "warn",
			fn:   Warn,
		},
		{
			desc: "error",
			fn:   Error,
		},
	}

	ctx := context.Background()
	SetLevel(DebugLevel)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			be, err := NewEventlogBackend(33, "guest-agent")
			if err != nil {
				t.Fatalf("NewEventlogBackend() failed: %v", err)
			}

			be.metrics = new(eventlogMetrics)
			RegisterBackend(ctx, be)

			test.fn("foobar")
			// The event log backend takes a while to set up, so keep waiting until
			// it's successful.
			start := time.Now()
			for be.metrics.success != 1 {
				if time.Since(start) >= 5*time.Second {
					t.Fatal("Timed out waiting for event log")
				}
				time.Sleep(10 * time.Millisecond)
			}
			Shutdown(time.Millisecond * 10)
		})
	}
}
