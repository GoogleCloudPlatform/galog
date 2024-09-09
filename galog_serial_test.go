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
	"testing"

	"go.bug.st/serial"
)

func TestSerialInvalidPort(t *testing.T) {
	tl := newTestLogger()
	be := NewSerialBackend(context.Background(), &SerialOptions{
		Port: "COM1",
		Baud: DefaultSerialBaud,
	})

	tl.RegisterBackend(context.Background(), be)
	ops := []func(args ...any){
		tl.Debug, tl.Info, tl.Warn, tl.Error,
	}

	for _, curr := range ops {
		curr("foobar")
	}
}

func TestSerialValidPort(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		t.Fatalf("serial.GetPortsList() failed: %v", err)
	}

	if len(ports) == 0 {
		t.Fatalf("serial.GetPortsList() returned no ports")
	}

	tl := newTestLogger()
	be := NewSerialBackend(context.Background(), &SerialOptions{
		Port: ports[0],
		Baud: DefaultSerialBaud,
	})

	tl.RegisterBackend(context.Background(), be)
	ops := []func(args ...any){
		tl.Debug, tl.Info, tl.Warn, tl.Error,
	}

	for _, curr := range ops {
		curr("foobar")
	}
}
