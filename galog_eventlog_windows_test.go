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
			Shutdown(time.Millisecond * 10)

			if be.metrics.success != 1 {
				t.Fatalf("success got %d, want 1", be.metrics.success)
			}
		})
	}
}
