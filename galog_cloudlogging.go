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
	"fmt"
	"time"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"google.golang.org/api/option"
)

// CloudLoggingInitMode is the cloud logging backend initialization mode.
type CloudLoggingInitMode int

const (
	// CloudLoggingInitModeLazy is the lazy initialization mode. In this mode the
	// backend object is created but the cloud logging client and logger are not
	// initialized until the first call to InitClient.
	CloudLoggingInitModeLazy CloudLoggingInitMode = iota
	// CloudLoggingInitModeActive is the active initialization mode. In this mode
	// the backend object is created and the cloud logging client and logger are
	// initialized immediately.
	CloudLoggingInitModeActive
	// DefaultCloudLoggingPingTimeout is the default timeout for pinging cloud
	// logging.
	DefaultCloudLoggingPingTimeout = time.Second
	// DefaultClientErrorInterval is the default interval for logging the client
	// errors.
	DefaultClientErrorInterval = time.Minute * 20
	// DefaultClientErrorLoggingDisabled is whether to enable the client error
	// logging by default.
	DefaultClientErrorLoggingDisabled = false
	// defaultCloudLoggingQueueSize is the default queue size of the cloud logging
	// backend implementation. In general writing to cloud logging should not
	// require buffering as cloud logging is async and logging library takes care
	// of flushing.
	defaultCloudLoggingQueueSize = 100
)

var (
	// errCloudLoggingNotInitialized is the error returned when the cloud
	// logging backend is not yet initialized.
	errCloudLoggingNotInitialized = errors.New("cloud logging logger is not yet fully initialized")

	// errCloudLoggingAlreadyInitialized is the error returned when the InitClient
	// is called and  cloud logging backend is already initialized.
	errCloudLoggingAlreadyInitialized = errors.New("cloud logging logger is already initialized")
)

// CloudBackend is a Backend implementation for cloud logging.
type CloudBackend struct {
	// backendID is the cloud logging backend implementation's ID.
	backendID string
	// client is the cloud logging client pointer.
	client *logging.Client
	// logger is the cloud logging logger pointer.
	logger *logging.Logger
	// config is a pointer to the generic Config interface implementation.
	config *backendConfig
	// opts is the cloud logging options.
	opts *CloudOptions
	// periodicLogger is the periodic logger used to capture the client errors.
	periodicLogger *periodicLogger
	// disableClientErrorLogging is whether to disable the client error logging.
	disableClientErrorLogging bool
}

// CloudOptions defines the cloud logging behavior and setup options.
type CloudOptions struct {
	// Ident is the logger's ident, or the logger's name.
	Ident string
	// ProgramName is the program name, it's used on the logging payload.
	ProgramName string
	// ProgramVersion is the program version, it's used on the logging payload.
	ProgramVersion string
	// Project the gcp project name.
	Project string
	// Instance the running instance name.
	Instance string
	// UserAgent is the logging user agent option.
	UserAgent string
	// FlushCadence is how frequently we should push the log to the server.
	FlushCadence time.Duration
	// ClientErrorInterval is how frequently we should log the client errors. This
	// defaults to [DefaultClientErrorInterval].
	ClientErrorInterval time.Duration
	// DisableClientErrorLogging is whether to disable the client error logging.
	// If enabled, any errors from the cloud logging client will be logged
	// periodically. Period is controlled by [ClientErrorInterval] option.
	// Periodical logging is used to prevent spamming the error logs in case of a
	// persistent error. By default this is enabled and controlled by
	// [DefaultClientErrorLoggingDisabled]. This periodic logger would attempt to
	// log at [WARN] level to every other configured backend (file, serial, etc).
	DisableClientErrorLogging bool
	// PingTimeout is the timeout for pinging Cloud Logging.
	//
	// This is required currently because the cloud logging flush operation hangs
	// indefinitely when the server is unreachable due to having no external IP
	// address or private Google access enabled. The timeout is used to attempt
	// pinging the Cloud Logging server, and if it's not reachable, we skip the
	// flush operation.
	PingTimeout time.Duration
	// WithoutAuthentication is whether to use authentication for cloud logging
	// operations.
	WithoutAuthentication bool
	// ExtraLabels are extra labels to be added to the cloud logging entry.
	ExtraLabels map[string]string
	// Endpoint is the cloud logging endpoint to use. If empty, the default
	// endpoint will be used.
	Endpoint string
}

// CloudEntryPayload contains the data to be sent to cloud logging as the
// entry payload. It's translated from the log subsystem's Entry structure.
type CloudEntryPayload struct {
	// Message is the formatted message.
	Message string `json:"message"`
	// LocalTimestamp is the unix timestamp got from the entry's When field.
	LocalTimestamp string `json:"localTimestamp"`
	// ProgName is the program name - or the binary name.
	ProgName string `json:"progName,omitempty"`
	// ProgVersion is the program version.
	ProgVersion string `json:"progVersion,omitempty"`
}

// NewCloudBackend returns a Backend implementation that will log out to google
// cloud logging.
//
// Initialization Mode:
//
// If mode is InitModeLazy the backend object will be allocated and only the
// the basic elements will be initialized, all log entries will be queued
// until InitClient is called.
//
// Why lazy initialization is important/needed?
//
// The Cloud Logging depends on instance name that's mainly a data fed by - or
// accessed from - metadata server and depending on the application and
// environment the metadata server might not be available at the time of the
// application start - being only available later on. In such cases the early
// initializing and registering the backend will result into entries being
// queued and written/sent to the cloud logging later on - that way no logging
// entries are lost.
func NewCloudBackend(ctx context.Context, mode CloudLoggingInitMode, opts *CloudOptions) (*CloudBackend, error) {
	res := &CloudBackend{
		backendID: "log-backend,cloudlogging",
		config:    newBackendConfig(defaultFileQueueSize),
	}

	res.config.SetFormat(ErrorLevel, `{{.Message}}`)
	res.config.SetFormat(DebugLevel, `{{.Message}}`)

	if mode == CloudLoggingInitModeActive {
		if err := res.InitClient(ctx, opts); err != nil {
			return nil, fmt.Errorf("failed to initialize cloud logging client: %+v", err)
		}
	}

	return res, nil
}

// InitClient initializes the cloud logging client and logger. If the backend
// was initialized in "active" mode this function is no-op.
func (cb *CloudBackend) InitClient(ctx context.Context, opts *CloudOptions) error {
	if cb.client != nil {
		return errCloudLoggingAlreadyInitialized
	}

	var clientOptions []option.ClientOption

	if opts.UserAgent != "" {
		clientOptions = append(clientOptions, option.WithUserAgent(opts.UserAgent))
	}

	if opts.WithoutAuthentication {
		clientOptions = append(clientOptions, option.WithoutAuthentication())
	}

	if opts.Endpoint != "" {
		clientOptions = append(clientOptions, option.WithEndpoint(opts.Endpoint))
	}

	// Set the default flush timeout if not provided.
	if opts.PingTimeout == 0 {
		opts.PingTimeout = DefaultCloudLoggingPingTimeout
	}

	cb.disableClientErrorLogging = opts.DisableClientErrorLogging

	errTimeout := opts.ClientErrorInterval
	if errTimeout == 0 {
		errTimeout = DefaultClientErrorInterval
	}

	cb.periodicLogger = &periodicLogger{
		interval: errTimeout,
	}

	client, err := logging.NewClient(ctx, opts.Project, clientOptions...)
	if err != nil {
		return fmt.Errorf("failed to initialize cloud logging client: %+v", err)
	}

	client.OnError = func(err error) {
		if opts.DisableClientErrorLogging {
			return
		}
		cb.periodicLogger.log(err)
	}

	labels := make(map[string]string)
	for k, v := range opts.ExtraLabels {
		labels[k] = v
	}
	if opts.Instance != "" {
		labels["instance_name"] = opts.Instance
	}

	var loggerOptions []logging.LoggerOption
	loggerOptions = append(loggerOptions, logging.CommonLabels(labels))
	loggerOptions = append(loggerOptions, logging.DelayThreshold(opts.FlushCadence))
	logger := client.Logger(opts.Ident, loggerOptions...)

	cb.client = client
	cb.logger = logger
	cb.opts = opts

	return nil
}

// Log prints sends the log entry to cloud logging.
func (cb *CloudBackend) Log(entry *LogEntry) error {
	// If the logger is nil it means the backend is lazy initialized, we return
	// an error to indicate that the backend is not yet initialized - the entries
	// will be queued.
	if cb.logger == nil {
		return errCloudLoggingNotInitialized
	}

	levelMap := map[Level]logging.Severity{
		FatalLevel:   logging.Critical,
		ErrorLevel:   logging.Error,
		WarningLevel: logging.Warning,
		InfoLevel:    logging.Info,
		DebugLevel:   logging.Debug,
	}

	severity := levelMap[entry.Level]
	sourceLocation := &logpb.LogEntrySourceLocation{
		File:     entry.File,
		Line:     int64(entry.Line),
		Function: entry.Function,
	}

	format := cb.config.Format(entry.Level)
	message, err := entry.Format(format)
	if err != nil {
		return fmt.Errorf("failed to format log message: %+v", err)
	}

	payload := &CloudEntryPayload{
		Message:        message,
		LocalTimestamp: entry.When.Format("2006-01-02T15:04:05.0000Z07:00"),
		ProgName:       cb.opts.ProgramName,
		ProgVersion:    cb.opts.ProgramVersion,
	}

	cb.logger.Log(logging.Entry{
		Severity:       severity,
		SourceLocation: sourceLocation,
		Payload:        payload,
	})

	return nil
}

// ID returns the cloud logging backend implementation's ID.
func (cb *CloudBackend) ID() string {
	return cb.backendID
}

// Config returns the configuration of the cloud logging backend.
func (cb *CloudBackend) Config() Config {
	return cb.config
}

// Shutdown forces the cloud logging backend to flush its content and closes the
// logging client. This operation is skipped if Cloud Logging is unreachable.
func (cb *CloudBackend) Shutdown(ctx context.Context) error {
	if cb.logger == nil {
		return errCloudLoggingNotInitialized
	}

	pingTimeout, cancelFunc := context.WithTimeout(ctx, cb.opts.PingTimeout)
	defer cancelFunc()

	// Ensure we can reach Cloud Logging before attempting to flush.
	if err := cb.client.Ping(pingTimeout); err != nil {
		return fmt.Errorf("failed to reach cloud logging, skipping flush: %v", err)
	}

	// Closing the client will flush the logs.
	if err := cb.client.Close(); err != nil {
		return fmt.Errorf("failed to close cloud logging client: %v", err)
	}
	return nil
}

// periodicLogger is a helper struct used to log the client errors periodically.
// Its to prevent spamming the logs with client errors. If there is a persistent
// error, the client will call the error handler too frequently.
type periodicLogger struct {
	// interval is the interval between logging the client errors.
	interval time.Duration
	// lastLog is the last time the client error was logged.
	lastLog time.Time
	// firstRunPassed is whether this is the first time the error handler was
	// called.
	firstRunPassed bool
}

// log logs the error if the interval has passed since the last log.
// Returns true if the error was logged, this is only used for testing.
func (pl *periodicLogger) log(err error) bool {
	if !pl.firstRunPassed || time.Since(pl.lastLog) >= pl.interval {
		pl.firstRunPassed = true
		pl.lastLog = time.Now()
		Warnf("Cloud Logging Client Error: %v", err)
		return true
	}
	return false
}
