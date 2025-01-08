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
	// CloudLoggingTimeout is the default timeout for the cloud logging backend
	// flush operation.
	CloudLoggingTimeout = 3 * time.Second
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
	// flushCadence is how frequently we should push the log to the server.
	FlushCadence time.Duration
	// WithoutAuthentication is whether to use authentication for cloud logging
	// operations.
	WithoutAuthentication bool
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

	client, err := logging.NewClient(ctx, opts.Project, clientOptions...)
	if err != nil {
		return fmt.Errorf("failed to initialize cloud logging client: %+v", err)
	}

	client.OnError = func(error) { return }
	var loggerOptions []logging.LoggerOption

	if opts.Instance != "" {
		labelOption := logging.CommonLabels(
			map[string]string{
				"instance_name": opts.Instance,
			},
		)
		loggerOptions = append(loggerOptions, labelOption)
	}

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

// Flush forces the cloud logging backend to flush its content.
func (cb *CloudBackend) Flush(ctx context.Context) error {
	if cb.logger == nil {
		return errCloudLoggingNotInitialized
	}

	if err := cb.client.Ping(ctx); err != nil {
		return fmt.Errorf("failed to reach cloud logging, skipping flush: %v", err)
	}

	if err := cb.logger.Flush(); err != nil {
		return fmt.Errorf("failed to flush cloud logging: %v", err)
	}
	return nil
}
