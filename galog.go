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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Config is the interface used to bridge behavior configurations
// between the log framework and the backend implementation.
type Config interface {
	// QueueSize returns the max size of the queue for a given backend.
	QueueSize() int
	// SetQueueSize sets the max size of the queue for a given backend.
	SetQueueSize(size int)
	// SetFormat sets the log format of the specified level for a given backend.
	SetFormat(level Level, format string)
	// Format returns the log format of the specified level from a given backen.
	Format(level Level) string
}

// FormatMap wraps the level <-> format map type.
type FormatMap map[Level]string

// backendConfig is a common implementation of Config interface, more
// "sophisticated" backends may want to have their own implementation,
// for a generic and simple use case backendConfig should suffice.
type backendConfig struct {
	// queueSize is the log record/entry queue max size. If greater than 0
	// the queue will be rotated/collected when it hits queueSize; The queue
	// size will be unlimited otherwise.
	queueSize int
	// formatMap maps the log format for a given log level.
	// See [Config]'s Format() for more info.
	formatMap FormatMap
}

// Backend defines the interface of a backend implementation.
type Backend interface {
	// ID returns the backend's implementation ID.
	ID() string
	// Log is the entry point with the Backend implementation, the log
	// framework will use Log() to forward the logging entry to the backend.
	Log(entry *LogEntry) error
	// Config returns the backend configuration interface implementation.
	Config() Config
	// Flush flushes the backend's backing log storage.
	Flush() error
}

// LogEntry describes a log record.
type LogEntry struct {
	// Level is the log level of the log record/entry.
	Level Level
	// File is the file name of the log caller.
	File string
	// Line is the file's line of the log caller.
	Line int
	// Function is the function name of the log caller.
	Function string
	// When is the time when this log record/entry was created.
	When time.Time
	// Message is the formated final log message.
	Message string
	// Prefix is a string/tag prefixed to the log message.
	Prefix string
}

// BackendQueue wraps all the entry queueing control objects.
type BackendQueue struct {
	// entries is the slice of queued log entries.
	entries []*LogEntry
	// entriesMutex protects access to entries variable. There will be two go
	// routine accessing entries, one processing the entries being enqueued with a
	// channel and another handling the periodic processing of enqueued entries.
	entriesMutex sync.Mutex
	// cancel is the channel used to cancel the queue handing go routine
	// of a given backend.
	cancel chan bool
	// bus is the channel used to enqueue a new log record/entry.
	bus chan *LogEntry
	// ticker is a timer used to periodically process pending queue.
	ticker *time.Ticker
	// tickerFrequency is the frequency of the ticker.
	tickerFrequency time.Duration
	// backend points to the actual backend object.
	backend Backend
}

// logger is the backing implementation of the logging facilities.
type logger struct {
	// currentLevel is the min log level currently set.
	currentLevel Level

	// queues packs the registered backend control data (indexed by backend ID).
	queues map[string]*BackendQueue

	// queuesMutex protects/syncs access to queues mapping. This mutex makes sure
	// it's thread safe to RegisterBackend() and UnregisterBackend() new backend's
	// queues as well as changing configurations such as SetQueueRetryFrequency().
	queuesMutex sync.Mutex

	// retryFrequency is the frequency for the log queue timed processing retry.
	retryFrequency time.Duration

	// verbosity is the verbosity level set for the logger.
	verbosity int

	// Prefix is a string/tag prefixed to the log message.
	prefix string

	// exitFunc is the exit function called on behalf Fatal and Fatalf calls.
	exitFunc func()
}

// Verbose is the verbosity controlled log operations.
type Verbose bool

var (
	// defaultLogger is the default logger used by the application.
	defaultLogger *logger

	// defaultRetryFrequency is the default frequency for the log queue timed
	// processing retry.
	defaultRetryFrequency = time.Millisecond * 10

	// FatalLevel is the log level definition for Fatal severity.
	FatalLevel = Level{0, "FATAL"}

	// ErrorLevel is the log level definition for Error severity.
	ErrorLevel = Level{1, "ERROR"}

	// WarningLevel is the log level definition for Warning severity.
	WarningLevel = Level{2, "WARNING"}

	// InfoLevel is the log level definition for Info severity.
	InfoLevel = Level{3, "INFO"}

	// DebugLevel is the log level definition for Deug severity.
	DebugLevel = Level{4, "DEBUG"}

	// allLevels is the list of all supported log levels.
	allLevels = []Level{FatalLevel, ErrorLevel, WarningLevel, InfoLevel, DebugLevel}
)

const (
	// fallbackFormat is used when a backend didn't provide the level <-> format
	// mapping.
	fallbackFormat = `{{.When.Format "2006-01-02T15:04:05.0000Z07:00"}} [{{.Level}}]: {{.Message}}`
)

func newLogger(exitFunc func()) *logger {
	return &logger{
		currentLevel:   WarningLevel,
		queues:         make(map[string]*BackendQueue),
		retryFrequency: defaultRetryFrequency,
		exitFunc:       exitFunc,
	}
}

// init initializes the default logger.
func init() {
	defaultLogger = newLogger(func() { os.Exit(1) })
}

// Level wraps id and description of a log level.
type Level struct {
	// level is the log level numeric id.
	level int
	// tag is the tag to be displayed when writing the log.
	tag string
}

// String returns the string representation of a log level.
func (level Level) String() string {
	return level.tag
}

// ParseLevel returns the log level object for a given level id. In case of
// invalid level id, an error is returned.
func ParseLevel(level int) (Level, error) {
	for _, lvl := range allLevels[1:] {
		if lvl.level == level {
			return lvl, nil
		}
	}
	return Level{level: level, tag: "INVALID"}, fmt.Errorf("invalid log level: %d", level)
}

// ValidLevels returns a string representation of all the valid log levels, the
// only exception is the Fatal level as it's a special internal level.
func ValidLevels() string {
	var levels []string
	// Iterate over all valid levels skipping fatal.
	for _, lvl := range allLevels[1:] {
		levels = append(levels, fmt.Sprintf("%s(%d)", lvl.tag, lvl.level))
	}
	return strings.Join(levels, ", ")
}

// SetLevel sets the current log level.
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// CurrentLevel returns the default logger current log level.
func CurrentLevel() Level {
	return defaultLogger.CurrentLevel()
}

// RegisterBackend inserts/registers a backend implementation. This function is
// thread safe and can be called from any goroutine in the program.
func RegisterBackend(ctx context.Context, backend Backend) {
	defaultLogger.RegisterBackend(ctx, backend)
}

// RegisteredBackendIDs returns the list of registered backend IDs.
func RegisteredBackendIDs() []string {
	return defaultLogger.RegisteredBackendIDs()
}

// UnregisterBackend removes/unregisters a backend implementation. This function
// is thread safe and can be called from any goroutine in the program.
func UnregisterBackend(backend Backend) {
	defaultLogger.UnregisterBackend(backend)
}

// Shutdown shuts down the default logger.
func Shutdown(timeout time.Duration) {
	defaultLogger.Shutdown(timeout)
}

// SetPrefix sets the prefix to be used for the log message. When present the
// default backend's formats will prefix the provided string to all log
// messages.
//
// In cases where multiple backends are writing to the same backing
// storage (i.e. same file or the same syslog's ident) the prefix adds a
// meaningful context of the log originator.
func SetPrefix(prefix string) {
	defaultLogger.SetPrefix(prefix)
}

// SetQueueRetryFrequency sets the frequency which the log queue will be
// processed. This function is thread safe and can be called from any goroutine
// in the program.
func SetQueueRetryFrequency(frequency time.Duration) {
	defaultLogger.SetQueueRetryFrequency(frequency)
}

// SetMinVerbosity sets the min verbosity to be assumed/used across the V()
// calls.
func SetMinVerbosity(v int) {
	defaultLogger.SetMinVerbosity(v)
}

// MinVerbosity returns an integer representing the currently min verbosity set.
func MinVerbosity() int {
	return defaultLogger.MinVerbosity()
}

// QueueRetryFrequency returns the currently log queue processing frequency.
func QueueRetryFrequency() time.Duration {
	return defaultLogger.QueueRetryFrequency()
}

// V returns a boolean of type Verbose, it's value is true if the requested
// level is less or equal to the min verbosity value (i.e. by passing -V flag).
// The returned value implements the logging functions such as Debug(),
// Debugf(), Info(), Infof() etc.
func V(v int) Verbose {
	return defaultLogger.V(v)
}

// Debug logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Debug(args ...any) {
	defaultLogger.Debug(args...)
}

// Debugf logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Debugf(format string, args ...any) {
	defaultLogger.Debugf(format, args...)
}

// Info logs to the INFO log. Arguments are handled in the manner of fmt.Print;
// New line adding is handled by each backend and will be added when it makes
// sense considering the backend context.
func Info(args ...any) {
	defaultLogger.Info(args...)
}

// Infof logs to the INFO log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Infof(format string, args ...any) {
	defaultLogger.Infof(format, args...)
}

// Warn logs to the WARNING log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Warn(args ...any) {
	defaultLogger.Warn(args...)
}

// Warnf logs to the WARNING log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Warnf(format string, args ...any) {
	defaultLogger.Warnf(format, args...)
}

// Error logs to the ERROR log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Error(args ...any) {
	defaultLogger.Error(args...)
}

// Errorf logs to the ERROR log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func Errorf(format string, args ...any) {
	defaultLogger.Errorf(format, args...)
}

// Fatal logs to the FATAL log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context. The Fatal function also
// exists the program with os.Exit(1).
//
// Since it's calling os.Exit() the fatal functions will also Shutdown() the
// logger specifying the queue retry frequency as the timeout
// (see [SetQueueRetryFrequency]).
func Fatal(args ...any) {
	defaultLogger.Fatal(args...)
}

// Fatalf logs to the FATAL log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.  The Fatal function also
// exists the program with os.Exit(1).
//
// Since it's calling os.Exit() the fatal functions will also Shutdown() the
// logger specifying the queue retry frequency as the timeout
// (see [SetQueueRetryFrequency]).
func Fatalf(format string, args ...any) {
	defaultLogger.Fatalf(format, args...)
}

// Debug logs to the DEBUG log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Print; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (v Verbose) Debug(args ...any) {
	defaultLogger.DebugV(v, args...)
}

// Debugf logs to the DEBUG log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Printf; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (v Verbose) Debugf(format string, args ...any) {
	defaultLogger.DebugfV(v, format, args...)
}

// Info logs to the INFO log assumed that the Verbosity v is smaller or equal to
// the configured min verbosity (see [V] for more). Arguments are handled in the
// manner of fmt.Print; New line adding is handled by each backend and will be
// added when it makes sense considering the backend context.
func (v Verbose) Info(args ...any) {
	defaultLogger.InfoV(v, args...)
}

// Infof logs to the INFO log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Printf; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (v Verbose) Infof(format string, args ...any) {
	defaultLogger.InfofV(v, format, args...)
}

// Warn logs to the WARNING log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Print; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (v Verbose) Warn(args ...any) {
	defaultLogger.WarnV(v, args...)
}

// Warnf logs to the WARNING log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Printf; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (v Verbose) Warnf(format string, args ...any) {
	defaultLogger.WarnfV(v, format, args...)
}

// Error logs to the ERROR log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Print; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (v Verbose) Error(args ...any) {
	defaultLogger.ErrorV(v, args...)
}

// Errorf logs to the ERROR log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Printf; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (v Verbose) Errorf(format string, args ...any) {
	defaultLogger.ErrorfV(v, format, args...)
}

// newEntry sets up the log entry for each logging call.
func newEntry(level Level, prefix string, msg string) *LogEntry {
	pc, file, line, _ := runtime.Caller(3)
	return &LogEntry{
		Level:    level,
		When:     time.Now(),
		File:     filepath.Base(file),
		Line:     line,
		Function: runtime.FuncForPC(pc).Name(),
		Message:  msg,
		Prefix:   prefix,
	}
}

// Format processes a template provided in format and return it as a string.
func (en *LogEntry) Format(format string) (string, error) {
	tmpl, err := template.New("").Parse(format)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	buffer := new(strings.Builder)
	if err := tmpl.Execute(buffer, en); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buffer.String(), nil
}

// newBackendConfig allocates and initializes a new backendConfig instance.
func newBackendConfig(queueSize int) *backendConfig {
	return &backendConfig{
		queueSize: queueSize,
		formatMap: make(FormatMap),
	}
}

// QueueSize returns the queueSize set to the configuration.
func (bc *backendConfig) QueueSize() int {
	return bc.queueSize
}

// SetQueueSize sets the size as the queueSize of the configuration. When a
// backend's queue size reaches its limit the oldest messages will start to be
// dropped one-by-one, the queue processing is asynchronous and
// thread safe - it runs on its own goroutine and synchronizes with the callers
// go routine using a channel. Resetting the queue size is also thread safe.
func (bc *backendConfig) SetQueueSize(size int) {
	bc.queueSize = size
}

// SetFormat adds level and format to the format mapping.
func (bc *backendConfig) SetFormat(level Level, format string) {
	bc.formatMap[level] = format
}

// Format returns the format configured to a given level. If n format is not
// found for the requested level the closest format will be used, i.e:
//
//   - if Error is found in the mapping, Info is not defined, and level is Info
//     the format of Error will be returned.
//
// If no format could be found, meaning, the backend has never defined a valid
// format configuration, a default fallbackFormat is returned.
func (bc *backendConfig) Format(level Level) string {
	if format, found := bc.formatMap[level]; found {
		return format
	}

	for i := level.level; i >= 0; i-- {
		curr := allLevels[i]
		if format, found := bc.formatMap[curr]; found {
			return format
		}
	}

	for i := level.level; i < len(allLevels); i++ {
		curr := allLevels[i]
		if format, found := bc.formatMap[curr]; found {
			return format
		}
	}

	return fallbackFormat
}

// SetPrefix sets the prefix to be used for the log message. When present the
// default backend's formats will prefix the provided string to all log
// messages.
//
// In cases where multiple backends are writing to the same backing
// storage (i.e. same file or the same syslog's ident) the prefix adds a
// meaningful context of the log originator.
func (lg *logger) SetPrefix(prefix string) {
	lg.prefix = prefix
}

// SetQueueRetryFrequency sets the frequency which the log queue will be
// processed.
func (lg *logger) SetQueueRetryFrequency(frequency time.Duration) {
	lg.retryFrequency = frequency

	lg.queuesMutex.Lock()
	for _, backend := range lg.queues {
		if backend.tickerFrequency != frequency {
			backend.tickerFrequency = frequency
			backend.ticker.Reset(frequency)
		}
	}
	lg.queuesMutex.Unlock()
}

// QueueRetryFrequency returns the currently set log queue processing frequency.
func (lg *logger) QueueRetryFrequency() time.Duration {
	return lg.retryFrequency
}

// UnregisterBackend removes/unregisters a backend implementation.
func (lg *logger) UnregisterBackend(backend Backend) {
	lg.queuesMutex.Lock()
	defer lg.queuesMutex.Unlock()
	if queue, found := lg.queues[backend.ID()]; found {
		delete(lg.queues, backend.ID())
		queue.ticker.Stop()
		queue.cancel <- true
	}
}

// Shutdown shuts down the logger. It unregisters all previously registered
// backends. Tries to flush out any pending enqueued entries for a period
// provided by the caller with timeout parameter.
//
// Any pending write in the backing storage of a backend will be flushed - the
// backend implementation's Flush() operation will be called.
//
// After calling Shutdown() all logging calls (i.e. Errorf(), Infof()) will be
// no-op (as no backends will left registered after that).
func (lg *logger) Shutdown(timeout time.Duration) {
	var backends []*BackendQueue

	lg.queuesMutex.Lock()
	for _, queue := range lg.queues {
		backends = append(backends, queue)
	}
	lg.queuesMutex.Unlock()

	// Equivalent to unsubscribe, make sure we stop accepting new entries.
	for _, queue := range backends {
		lg.UnregisterBackend(queue.backend)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Flush any pending writes.
	for _, queue := range backends {
		if ctx.Err() != nil {
			break
		}
		lg.flushEnqueuedEntries(ctx, queue)
		queue.backend.Flush()
	}
}

// RegisterBackend inserts/registers a backend implementation.
func (lg *logger) RegisterBackend(ctx context.Context, backend Backend) {
	backendQueue := &BackendQueue{
		cancel:          make(chan bool),
		bus:             make(chan *LogEntry),
		tickerFrequency: lg.retryFrequency,
		ticker:          time.NewTicker(lg.retryFrequency),
		backend:         backend,
	}

	lg.queuesMutex.Lock()
	lg.queues[backend.ID()] = backendQueue
	lg.queuesMutex.Unlock()

	go lg.runBackend(ctx, backend, backendQueue)
}

// RegisteredBackendIDs returns the list of registered backend IDs.
func (lg *logger) RegisteredBackendIDs() []string {
	lg.queuesMutex.Lock()
	defer lg.queuesMutex.Unlock()
	var backendIDs []string
	for _, queue := range lg.queues {
		backendIDs = append(backendIDs, queue.backend.ID())
	}
	return backendIDs
}

// runBackend runs the go routine receiving signals to write a log from a
// backend or ultimately enqueuing it. Each registered backend will have its own
// enqueuing managing go routine.
func (lg *logger) runBackend(ctx context.Context, backend Backend, bq *BackendQueue) {
	// enqueue guarantees the max size of the entry queue.
	enqueue := func(force bool, config Config, item *LogEntry) bool {
		bq.entriesMutex.Lock()
		defer bq.entriesMutex.Unlock()

		if !force && len(bq.entries) == 0 {
			return false
		}

		if config == nil || config.QueueSize() == 0 {
			return false
		}

		bq.entries = append(bq.entries, item)
		queueSize := config.QueueSize()
		// If we've reached the queue limit we remove the oldest entry from the
		// queue.
		if queueSize > 0 && len(bq.entries) > queueSize {
			bq.entries = bq.entries[1:]
		}

		return true
	}

	var wg sync.WaitGroup

	wg.Add(1)
	// Runs the the periodical processing of the queue.
	go func() {
		defer wg.Done()
		for {
			select {
			// Context cancelation handling.
			case <-ctx.Done():
				bq.ticker.Stop()
				return
			// Periodically queue processing.
			case <-bq.ticker.C:
				lg.flushEnqueuedEntries(ctx, bq)
			}
		}
	}()

	wg.Add(1)
	// Runs the entry enqueueing and backend unregistration.
	go func() {
		defer wg.Done()
		for {
			select {
			// Context cancelation handling.
			case <-ctx.Done():
				return
			// Backend unregistering handling.
			case <-bq.cancel:
				return
			// Entry enqueueing handling.
			case entry := <-bq.bus:
				if enqueue(false, backend.Config(), entry) {
					continue
				}
				if err := backend.Log(entry); err != nil {
					enqueue(true, backend.Config(), entry)
				}
			}
		}
	}()

	wg.Wait()
}

// flushEnqueuedEntries try to write any pending entries from the queue. It
// will attempt to write until the context is canceled otherwise it runs until
// all the queue is processed.
func (lg *logger) flushEnqueuedEntries(ctx context.Context, bq *BackendQueue) {
	var success int
	bq.entriesMutex.Lock()
	defer bq.entriesMutex.Unlock()
	for _, curr := range bq.entries {
		if ctx.Err() != nil {
			break
		}

		if err := bq.backend.Log(curr); err != nil {
			break
		}
		success++
	}
	if len(bq.entries) > 0 && success > 0 {
		bq.entries = bq.entries[success:]
	}
}

// log is the bridging point between the logging functions and the
// queue management layer. Each backend will have their own queue.
// If the log entry level is not higher than the currently set level
// the entry is ignored.
func (lg *logger) log(entry *LogEntry) {
	if lg.currentLevel.level < entry.Level.level {
		return
	}

	lg.queuesMutex.Lock()
	defer lg.queuesMutex.Unlock()

	for _, curr := range lg.queues {
		curr.bus <- entry
	}
}

// SetLevel sets the current log level of lg.
func (lg *logger) SetLevel(level Level) {
	lg.currentLevel = level
}

// CurrentLevel returns the current log level of lg.
func (lg *logger) CurrentLevel() Level {
	return lg.currentLevel
}

// Debug logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Debug(args ...any) {
	lg.log(newEntry(DebugLevel, lg.prefix, fmt.Sprint(args...)))
}

// Debugf logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Debugf(format string, args ...any) {
	lg.log(newEntry(DebugLevel, lg.prefix, fmt.Sprintf(format, args...)))
}

// Info logs to the INFO log. Arguments are handled in the manner of fmt.Print;
// New line adding is handled by each backend and will be added when it makes
// sense considering the backend context.
func (lg *logger) Info(args ...any) {
	lg.log(newEntry(InfoLevel, lg.prefix, fmt.Sprint(args...)))
}

// Infof logs to the INFO log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Infof(format string, args ...any) {
	lg.log(newEntry(InfoLevel, lg.prefix, fmt.Sprintf(format, args...)))
}

// Warn logs to the WARNING log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Warn(args ...any) {
	lg.log(newEntry(WarningLevel, lg.prefix, fmt.Sprint(args...)))
}

// Warnf logs to the WARNING log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Warnf(format string, args ...any) {
	lg.log(newEntry(WarningLevel, lg.prefix, fmt.Sprintf(format, args...)))
}

// Error logs to the ERROR log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Error(args ...any) {
	lg.log(newEntry(ErrorLevel, lg.prefix, fmt.Sprint(args...)))
}

// Errorf logs to the ERROR log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.
func (lg *logger) Errorf(format string, args ...any) {
	lg.log(newEntry(ErrorLevel, lg.prefix, fmt.Sprintf(format, args...)))
}

// Fatal logs to the FATAL log. Arguments are handled in the manner of
// fmt.Print; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context. The Fatal function also
// exists the program with os.Exit(1).
//
// Since it's calling os.Exit() the fatal functions will also Shutdown() the
// logger specifying the queue retry frequency as the timeout
// (see [SetQueueRetryFrequency]).
func (lg *logger) Fatal(args ...any) {
	lg.log(newEntry(FatalLevel, lg.prefix, fmt.Sprint(args...)))
	lg.Shutdown(lg.retryFrequency)
	lg.exitFunc()
}

// Fatalf logs to the FATAL log. Arguments are handled in the manner of
// fmt.Printf; New line adding is handled by each backend and will be added when
// it makes sense considering the backend context.  The Fatal function also
// exists the program with os.Exit(1).
//
// Since it's calling os.Exit() the fatal functions will also Shutdown() the
// logger specifying the queue retry frequency as the timeout
// (see [SetQueueRetryFrequency]).
func (lg *logger) Fatalf(format string, args ...any) {
	lg.log(newEntry(FatalLevel, lg.prefix, fmt.Sprintf(format, args...)))
	lg.Shutdown(lg.retryFrequency)
	lg.exitFunc()
}

// SetMinVerbosity sets the min verbosity to be assumed/used across the V()
// calls.
func (lg *logger) SetMinVerbosity(v int) {
	lg.verbosity = v
}

// V returns a boolean of type Verbose, it's value is true if the requested
// level is less or equal to the min verbosity value (i.e. by passing -V flag).
// The returned value implements the logging functions such as Debug(),
// Debugf(), Info(), Infof() etc.
func (lg *logger) V(v int) Verbose {
	if v <= lg.verbosity {
		return true
	}
	return false
}

// MinVerbosity returns an integer representing the currently min verbosity set.
func (lg *logger) MinVerbosity() int {
	return defaultLogger.verbosity
}

// DebugV logs to the DEBUG log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Print; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (lg *logger) DebugV(v Verbose, args ...any) {
	if v {
		lg.log(newEntry(DebugLevel, lg.prefix, fmt.Sprint(args...)))
	}
}

// DebugfV logs to the DEBUG log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Printf; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (lg *logger) DebugfV(v Verbose, format string, args ...any) {
	if v {
		lg.log(newEntry(DebugLevel, lg.prefix, fmt.Sprintf(format, args...)))
	}
}

// InfoV logs to the INFO log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Print; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (lg *logger) InfoV(v Verbose, args ...any) {
	if v {
		lg.log(newEntry(InfoLevel, lg.prefix, fmt.Sprint(args...)))
	}
}

// InfofV logs to the INFO log assumed that the Verbosity v is smaller or equal
// to the configured min verbosity (see [V] for more). Arguments are handled in
// the manner of fmt.Printf; New line adding is handled by each backend and will
// be added when it makes sense considering the backend context.
func (lg *logger) InfofV(v Verbose, format string, args ...any) {
	if v {
		lg.log(newEntry(InfoLevel, lg.prefix, fmt.Sprintf(format, args...)))
	}
}

// WarnV logs to the WARNING log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Print; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (lg *logger) WarnV(v Verbose, args ...any) {
	if v {
		lg.log(newEntry(WarningLevel, lg.prefix, fmt.Sprint(args...)))
	}
}

// WarnfV logs to the WARNING log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Printf; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (lg *logger) WarnfV(v Verbose, format string, args ...any) {
	if v {
		lg.log(newEntry(WarningLevel, lg.prefix, fmt.Sprintf(format, args...)))
	}
}

// ErrorV logs to the ERROR log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Print; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (lg *logger) ErrorV(v Verbose, args ...any) {
	if v {
		lg.log(newEntry(ErrorLevel, lg.prefix, fmt.Sprint(args...)))
	}
}

// ErrorfV logs to the ERROR log assumed that the Verbosity v is smaller or
// equal to the configured min verbosity (see [V] for more). Arguments are
// handled in the manner of fmt.Printf; New line adding is handled by each
// backend and will be added when it makes sense considering the backend
// context.
func (lg *logger) ErrorfV(v Verbose, format string, args ...any) {
	if v {
		lg.log(newEntry(ErrorLevel, lg.prefix, fmt.Sprintf(format, args...)))
	}
}
