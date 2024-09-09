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
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

const (
	defaultBypassBackendID = "test-bypass-backend"
)

func newTestLogger() *logger {
	return newLogger(func() {})
}

type bypassBackend struct {
	mu     sync.Mutex
	id     string
	entry  *LogEntry
	config Config
}

func newBypassBackend(id string, queueSize int) *bypassBackend {
	return &bypassBackend{id: id, config: newBackendConfig(queueSize)}
}

func (bl *bypassBackend) fetchEntry() *LogEntry {
	var entry *LogEntry
	for i := 1; i <= 10; i++ {
		bl.mu.Lock()
		entry = bl.entry
		bl.mu.Unlock()
		if entry != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return entry
}

func (bl *bypassBackend) ID() string {
	return bl.id
}

func (bl *bypassBackend) Log(entry *LogEntry) error {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.entry = entry
	return nil
}

func (bl *bypassBackend) Config() Config {
	return bl.config
}

func (bl *bypassBackend) Flush() error {
	return nil
}

func TestRegisteredBackendIDs(t *testing.T) {
	tl := newTestLogger()
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	tests := []struct {
		name      string
		backendID string
	}{
		{"registeredBackendIDs=backend-bypass,0", "backend-bypass,0"},
		{"registeredBackendIDs=backend-bypass,1", "backend-bypass,1"},
		{"registeredBackendIDs=backend-bypass,2", "backend-bypass,2"},
	}

	globalLogger := defaultLogger
	defaultLogger = tl

	t.Cleanup(func() {
		Shutdown(time.Millisecond)
		defaultLogger = globalLogger
	})

	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bb := newBypassBackend(tc.backendID, 0)
			RegisterBackend(context.Background(), bb)

			ids := RegisteredBackendIDs()
			if len(ids) != index+1 {
				t.Errorf("len(RegisteredBackendIDs()) = %d, want: %d", len(ids), index+1)
			}

			if !slices.Contains(ids, tc.backendID) {
				t.Errorf("RegisteredBackendIDs() = %v, should contain: %v", ids, tc.backendID)
			}
		})
	}
}

func TestNonFormat(t *testing.T) {
	tl := newTestLogger()
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	testArguments := []any{"foobar"}
	expectedMessage := "foobar"

	tests := []struct {
		desc          string
		fc            func(args ...any)
		expectedLevel Level
	}{
		{
			desc:          "debug",
			fc:            tl.Debug,
			expectedLevel: DebugLevel,
		},
		{
			desc:          "info",
			fc:            tl.Info,
			expectedLevel: InfoLevel,
		},
		{
			desc:          "warning",
			fc:            tl.Warn,
			expectedLevel: WarningLevel,
		},
		{
			desc:          "error",
			fc:            tl.Error,
			expectedLevel: ErrorLevel,
		},
		{
			desc:          "fatal",
			fc:            tl.Fatal,
			expectedLevel: FatalLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			bb := newBypassBackend("bypass-non-format", 0)
			tl.RegisterBackend(context.Background(), bb)
			t.Cleanup(func() {
				tl.UnregisterBackend(bb)
			})

			tc.fc(testArguments...)
			entry := bb.fetchEntry()

			if entry == nil {
				t.Error("bb.entry = nil, want: non-nil")
			} else if entry.Level != tc.expectedLevel {
				t.Errorf("bb.entry.Level = %s, want: %s", bb.entry.Level, tc.expectedLevel)
			} else if entry.Message != expectedMessage {
				t.Errorf("bb.entry.Message = %s, want: %s", bb.entry.Message, expectedMessage)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tl := newTestLogger()
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	testFormat := "%s: %d"
	testArguments := []any{"foobar", 33}
	expectedMessage := "foobar: 33"

	tests := []struct {
		desc          string
		fc            func(format string, args ...any)
		expectedLevel Level
	}{
		{
			desc:          "debug",
			fc:            tl.Debugf,
			expectedLevel: DebugLevel,
		},
		{
			desc:          "info",
			fc:            tl.Infof,
			expectedLevel: InfoLevel,
		},
		{
			desc:          "warning",
			fc:            tl.Warnf,
			expectedLevel: WarningLevel,
		},
		{
			desc:          "error",
			fc:            tl.Errorf,
			expectedLevel: ErrorLevel,
		},
		{
			desc:          "fatal",
			fc:            tl.Fatalf,
			expectedLevel: FatalLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			bb := newBypassBackend("bypass-format", 0)
			tl.RegisterBackend(context.Background(), bb)
			t.Cleanup(func() { tl.UnregisterBackend(bb) })

			tc.fc(testFormat, testArguments...)
			entry := bb.fetchEntry()

			if entry == nil {
				t.Error("bb.entry = nil, want: non-nil")
			} else if entry.Level != tc.expectedLevel {
				t.Errorf("bb.entry.Level = %s, want: %s", bb.entry.Level, tc.expectedLevel)
			} else if entry.Message != expectedMessage {
				t.Errorf("bb.entry.Message = %s, want: %s", bb.entry.Message, expectedMessage)
			}
		})
	}
}

func TestFormatHierarchy(t *testing.T) {
	tl := newTestLogger()
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	be := newBypassBackend(defaultBypassBackendID, 0)
	tl.RegisterBackend(context.Background(), be)
	t.Cleanup(func() {
		tl.UnregisterBackend(be)
	})

	cfg := be.Config()

	cfg.SetFormat(DebugLevel, "[DEBUG] {{.Message}}")

	tl.Info("foobar")

	format := cfg.Format(DebugLevel)

	entry := be.fetchEntry()
	if entry == nil {
		t.Fatal("be.entry = nil, want: non-nil")
	}

	msg, err := entry.Format(format)

	if err != nil {
		t.Fatalf("should have succeeded, returned error: %+v", err)
	}

	if msg != "[DEBUG] foobar" {
		t.Fatalf("got: %s, expected: %s", msg, "[DEBUG] foobar")
	}

}

type enqueuedBackend struct {
	mu             sync.RWMutex
	entries        []string
	defaultRetries int
	retries        int
	config         Config
}

func newEnqueuedBackend(queueSize int, defaultRetries int) *enqueuedBackend {
	return &enqueuedBackend{
		defaultRetries: defaultRetries,
		retries:        defaultRetries,
		config:         newBackendConfig(queueSize),
	}
}

func (eb *enqueuedBackend) ID() string {
	return "test-enqueued-backend"
}

func (eb *enqueuedBackend) fetchEntries() []string {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.entries
}

func (eb *enqueuedBackend) Log(entry *LogEntry) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.retries == 0 {
		eb.entries = append(eb.entries, entry.Message)
		eb.retries = eb.defaultRetries
		return nil
	}

	eb.retries--
	return fmt.Errorf("enqueuedBackend error")
}

func (eb *enqueuedBackend) Config() Config {
	return eb.config
}

func (eb *enqueuedBackend) Flush() error {
	return nil
}

func TestEnqueued(t *testing.T) {
	tests := []struct {
		desc    string
		message string
	}{
		{
			desc:    "enqueued_foobar",
			message: "foobar",
		},
		{
			desc:    "enqueued_foo-bar",
			message: "foo-bar",
		},
		{
			desc:    "enqueued_foo_bar",
			message: "foo bar",
		},
		{
			desc:    "enqueued_foo",
			message: "foo",
		},
		{
			desc:    "enqueued_bar",
			message: "bar",
		},
	}

	tl := newTestLogger()
	tl.SetQueueRetryFrequency(time.Millisecond)
	eb := newEnqueuedBackend(len(tests), 2)
	tl.RegisterBackend(context.Background(), eb)

	t.Cleanup(func() { tl.UnregisterBackend(eb) })

	for _, tc := range tests {
		tl.Error(tc.message)
	}

	time.Sleep(time.Second)
	entries := eb.fetchEntries()

	if len(entries) != len(tests) {
		t.Errorf("len(eb.entries) = %d, want: %d", len(eb.entries), len(tests))
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			entries := eb.fetchEntries()
			if !slices.Contains(entries, tc.message) {
				t.Errorf("log entry not found: %s", tc.message)
			}
		})
	}
}

func TestInvalidLogFormat(t *testing.T) {
	entry := newEntry(ErrorLevel, "", "foobar")
	_, err := entry.Format("{{.InvalidField}}")
	if err == nil {
		t.Fatal("should have returned error, succeeded instead")
	}
}

func TestValidLogFormat(t *testing.T) {
	entry := newEntry(ErrorLevel, "", "foobar")
	_, err := entry.Format("{{.Message}}")
	if err != nil {
		t.Fatalf("should have succeeded, returned error: %+v", err)
	}
}

func TestNoOpVerbosityFormat(t *testing.T) {
	tl := newTestLogger()
	bb := newBypassBackend(defaultBypassBackendID, 0)
	tl.SetQueueRetryFrequency(time.Millisecond)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.DebugfV(tl.V(1), "foo: %s", "bar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}

	tl.InfofV(tl.V(1), "foo: %s", "bar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}

	tl.WarnfV(tl.V(1), "foo: %s", "bar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}
}

func TestNoOpVerbosityNoFormat(t *testing.T) {
	tl := newTestLogger()
	bb := newBypassBackend(defaultBypassBackendID, 0)
	tl.SetQueueRetryFrequency(time.Millisecond)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.DebugV(tl.V(1), "foobar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}

	tl.InfoV(tl.V(1), "foobar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}

	tl.WarnV(tl.V(1), "foobar")
	if bb.entry != nil {
		t.Errorf("bb.entry = %+v, want: nil", bb.entry)
	}
}

func TestVerbosityNoFormat(t *testing.T) {
	tl := newTestLogger()
	tl.SetMinVerbosity(2)
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	bb := newBypassBackend(defaultBypassBackendID, 1)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.DebugV(tl.V(1), "foobar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil

	tl.InfoV(tl.V(1), "foobar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil

	tl.WarnV(tl.V(1), "foobar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil
}

func TestVerbosityFormat(t *testing.T) {
	tl := newTestLogger()
	tl.SetMinVerbosity(2)
	tl.SetLevel(DebugLevel)
	tl.SetQueueRetryFrequency(time.Millisecond)

	bb := newBypassBackend(defaultBypassBackendID, 1)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.DebugfV(tl.V(1), "foo: %s", "bar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil

	tl.InfofV(tl.V(1), "foo: %s", "bar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil

	tl.WarnfV(tl.V(1), "foo: %s", "bar")
	if bb.fetchEntry() == nil {
		t.Error("bb.entry = nil, want: non-nil")
	}
	bb.entry = nil
}

func TestSetMinVerbosity(t *testing.T) {
	defaultLogger = newTestLogger()
	defaultVerbosity := defaultLogger.verbosity
	defaultLogger.SetQueueRetryFrequency(time.Millisecond)
	t.Cleanup(func() { defaultLogger.verbosity = defaultVerbosity })

	SetMinVerbosity(100)

	if defaultLogger.verbosity != 100 {
		t.Errorf("verbosityFlag = %d, want: %d", defaultLogger.verbosity, 100)
	}
}

func TestGetMinVerbosity(t *testing.T) {
	defaultLogger = newTestLogger()
	if MinVerbosity() != defaultLogger.verbosity {
		t.Errorf("MinVerbosity() = %d, want: 0", MinVerbosity())
	}
}

func TestSetQueueRetryFrequency(t *testing.T) {
	defaultLogger = newTestLogger()
	retryFrequency := time.Second * 100
	defaultLogger.SetQueueRetryFrequency(time.Millisecond)
	bb := newBypassBackend(defaultBypassBackendID, 0)
	RegisterBackend(context.Background(), bb)

	oldRetryFrequency := QueueRetryFrequency()
	t.Cleanup(func() {
		SetQueueRetryFrequency(oldRetryFrequency)
		UnregisterBackend(bb)
	})

	SetQueueRetryFrequency(retryFrequency)

	if val := QueueRetryFrequency(); val != retryFrequency {
		t.Errorf("QueueRetryFreuency() = %s, want: %s", QueueRetryFrequency(), retryFrequency)
	}

	for i, curr := range defaultLogger.queues {
		t.Run(fmt.Sprintf("set-queue-retry-freq-%s", i), func(t *testing.T) {
			if curr.tickerFrequency != retryFrequency {
				t.Errorf("curr.tickerFrequency = %s, want: %s", curr.tickerFrequency, retryFrequency)
			}
		})
	}
}

func TestBackendConfigFormat(t *testing.T) {
	config := newBackendConfig(0)

	debugFormat := "[{{.Level}}]: ({{.File}}:{{.Line}}) {{.Message}}"
	config.SetFormat(DebugLevel, debugFormat)

	format := config.Format(DebugLevel)
	if format != debugFormat {
		t.Errorf("Format(%s) = %s, want: %s", DebugLevel, format, debugFormat)
	}

	format = config.Format(InfoLevel)
	if format != debugFormat {
		t.Errorf("format = %s, want: %s", format, debugFormat)
	}

	infoFormat := "[{{.Level}}]: {{.Message}}"
	config.SetFormat(InfoLevel, infoFormat)

	format = config.Format(InfoLevel)
	if format != infoFormat {
		t.Errorf("format = %s, want: %s", format, infoFormat)
	}
}

func TestLevelString(t *testing.T) {
	for _, curr := range allLevels {
		if curr.tag != curr.String() {
			t.Errorf("curr.tag = %s, want: %s", curr.tag, curr.String())
		}
	}
}

type flushBackend struct {
	shouldFail bool
	flushed    bool
	config     Config
}

func newFlushBackend(queueSize int, shouldFail bool) *flushBackend {
	return &flushBackend{flushed: false, config: newBackendConfig(queueSize), shouldFail: shouldFail}
}

func (bl *flushBackend) ID() string {
	return "test-flush-backend"
}

func (bl *flushBackend) Log(entry *LogEntry) error {
	return nil
}

func (bl *flushBackend) Config() Config {
	return bl.config
}

func (bl *flushBackend) Flush() error {
	if bl.shouldFail {
		return fmt.Errorf("flushBackend error")
	}
	bl.flushed = true
	return nil
}

func TestFlushSuccess(t *testing.T) {
	tl := newTestLogger()
	tl.SetQueueRetryFrequency(time.Millisecond)
	bb := newFlushBackend(1, false)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.Shutdown(time.Millisecond)

	if !bb.flushed {
		t.Errorf("bb.flushed = false, want: true")
	}
}

func TestFlushFailure(t *testing.T) {
	tl := newTestLogger()
	tl.SetQueueRetryFrequency(time.Millisecond)
	bb := newFlushBackend( /* queueSize= */ 1 /* shouldFail= */, true)
	tl.RegisterBackend(context.Background(), bb)
	t.Cleanup(func() { tl.UnregisterBackend(bb) })

	tl.Shutdown(time.Millisecond)

	if bb.flushed {
		t.Errorf("bb.flushed = true, want: false")
	}
}

func TestParseLevelSuccess(t *testing.T) {
	tests := []struct {
		name     string
		intVal   int
		expected *Level
	}{
		{"validLevel=Error", 1, &ErrorLevel},
		{"validLevel=Warning", 2, &WarningLevel},
		{"validLevel=Info", 3, &InfoLevel},
		{"validLevel=Debug", 4, &DebugLevel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseLevel(tc.intVal)
			if err != nil {
				t.Errorf("ParseLevel(%d) = %+v, want: nil", tc.intVal, err)
			}

			if parsed.level != tc.expected.level && parsed.tag != tc.expected.tag {
				t.Errorf("ParseLevel(%d) = %+v, want: %+v", tc.intVal, parsed, tc.expected)
			}
		})
	}
}

func TestParseLevelFailure(t *testing.T) {
	tests := []struct {
		name   string
		intVal int
	}{
		{"invalidLevel=0", 0},
		{"invalidLevel=5", 5},
		{"invalidLevel=6", 6},
		{"invalidLevel=7", 7},
		{"invalidLevel=8", 8},
		{"invalidLevel=9", 9},
		{"invalidLevel=10", 10},
	}
	for _, curr := range tests {
		t.Run(curr.name, func(t *testing.T) {
			parsed, err := ParseLevel(curr.intVal)
			if err == nil {
				t.Errorf("ParseLevel(%d) = (%+v, nil), want: (nil, nil)", curr.intVal, parsed)
			}
		})
	}
}

func TestValidLevels(t *testing.T) {
	expected := "ERROR(1), WARNING(2), INFO(3), DEBUG(4)"
	res := ValidLevels()
	if res != expected {
		t.Errorf("ValidLevels() = %s, want: %s", res, expected)
	}
}

func TestPrefix(t *testing.T) {
	bb := newBypassBackend("test-prefix", 0)
	RegisterBackend(context.Background(), bb)

	oldRetryFrequency := QueueRetryFrequency()
	oldExitFunc := defaultLogger.exitFunc

	defer t.Cleanup(func() {
		SetQueueRetryFrequency(oldRetryFrequency)
		SetPrefix("")
		defaultLogger.exitFunc = oldExitFunc
		UnregisterBackend(bb)
	})

	prefix := "application_name"
	SetPrefix(prefix)
	SetQueueRetryFrequency(time.Millisecond)
	defaultLogger.exitFunc = func() {}

	tests := []struct {
		name string
		logF func(format string, args ...any)
		log  func(args ...any)
	}{
		{"error_log", Errorf, Error},
		{"warning_log", Warnf, Warn},
		{"info_log", Infof, Info},
		{"debug_log", Debugf, Debug},
		{"fatal_log", Fatalf, Fatal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.logF("test prefix message, with prefix: %s", prefix)
			entry := bb.fetchEntry()
			if entry == nil {
				t.Fatalf("bb.entry = nil, want: non-nil")
			}

			if entry.Prefix != prefix {
				t.Errorf("bb.entry.Prefix = %s, want: %s", entry.Prefix, prefix)
			}

			tc.log("test prefix message")
			entry = bb.fetchEntry()
			if entry == nil {
				t.Fatalf("bb.entry = nil, want: non-nil")
			}

			if entry.Prefix != prefix {
				t.Errorf("bb.entry.Prefix = %s, want: %s", entry.Prefix, prefix)
			}
		})
	}
}

func TestVerbositySuccess(t *testing.T) {
	bb := newBypassBackend("test-verbosity", 0)
	RegisterBackend(context.Background(), bb)
	currLevel := CurrentLevel()

	oldRetryFrequency := QueueRetryFrequency()
	defer t.Cleanup(func() {
		SetLevel(currLevel)
		SetQueueRetryFrequency(oldRetryFrequency)
		UnregisterBackend(bb)
	})

	SetLevel(DebugLevel)
	SetQueueRetryFrequency(time.Millisecond)
	SetMinVerbosity(1)

	V(1).Error("verbosity = 1, error()")
	entry := bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Errorf("verbosity = 1, %s", "errorf()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Info("verbosity = 1, info()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Infof("verbosity = 1, %s", "infof()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Warn("verbosity = 1, warn()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Warnf("verbosity = 1, %s", "warnf()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Debug("verbosity = 1, debug()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}

	V(1).Debugf("verbosity = 1, %s", "debugf()")
	entry = bb.fetchEntry()
	if entry == nil {
		t.Errorf("entry = nil, want: non-nil")
	}
}

func TestVerbosityFailure(t *testing.T) {
	bb := newBypassBackend("test-verbosity", 0)
	RegisterBackend(context.Background(), bb)
	currLevel := CurrentLevel()

	oldRetryFrequency := QueueRetryFrequency()
	defer t.Cleanup(func() {
		SetLevel(currLevel)
		SetQueueRetryFrequency(oldRetryFrequency)
		UnregisterBackend(bb)
	})

	SetLevel(DebugLevel)
	SetQueueRetryFrequency(time.Millisecond)
	SetMinVerbosity(0)

	V(1).Error("verbosity = 1, error()")
	entry := bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Errorf("verbosity = 1, %s", "errorf()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Info("verbosity = 1, info()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Infof("verbosity = 1, %s", "infoff()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Warn("verbosity = 1, warn()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Warnf("verbosity = 1, %s", "warnf()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Debug("verbosity = 1, debug()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}

	V(1).Debugf("verbosity = 1, %s", "debugf()")
	entry = bb.fetchEntry()
	if entry != nil {
		t.Errorf("entry = non-nil, want: nil")
	}
}

func TestInvalidFormat(t *testing.T) {
	entry := newEntry(ErrorLevel, "", "foobar")

	_, err := entry.Format("{{.InvalidField")
	if err == nil {
		t.Fatal("entry.Format() = nil, want: error")
	}

	_, err = entry.Format("{{.InvalidField}}")
	if err == nil {
		t.Fatal("entry.Format() = nil, want: error")
	}
}

func TestFallbackFormat(t *testing.T) {
	cfg := newBackendConfig(10)
	msgFormat := cfg.Format(InfoLevel)
	if msgFormat != fallbackFormat {
		t.Fatalf("cfg.Format(%s) = %s, want: %s", InfoLevel, msgFormat, fallbackFormat)
	}
}

func TestSetBackendQueueSize(t *testing.T) {
	bb := newBypassBackend("test-queue-size", 0)
	currQueueSize := bb.Config().QueueSize()
	want := currQueueSize + 10
	bb.Config().SetQueueSize(want)
	if bb.Config().QueueSize() != want {
		t.Errorf("bb.Config().QueueSize() = %d, want: %d", bb.Config().QueueSize(), want)
	}
}
