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

import (
	"context"
	"testing"
	"time"
)

func TestNoopSyslogWindows(t *testing.T) {
	be := NewSyslogBackend("test_ident")
	be.metrics = new(syslogMetrics)
	RegisterBackend(context.Background(), be)

	entry := newEntry(ErrorLevel, "", "foobar")
	err := be.Log(entry)
	if err != nil {
		t.Fatalf("Log() = %v, want nil", err)
	}

	be.metrics.mu.Lock()
	ctr := be.metrics.success
	be.metrics.mu.Unlock()

	if ctr != 0 {
		t.Errorf("metrics.success = %d, want 0", ctr)
	}

	Shutdown(time.Millisecond)
}
