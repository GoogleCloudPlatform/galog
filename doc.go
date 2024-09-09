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

// Package galog implements a generic, highly configurable, queueable, with a
// plug-able backend architecture. By default galog provides a set of predefined
// backends, such as: [CloudBackend], [EventlogBackend], [FileBackend],
// [SyslogBackend] etc.
//
// # Initialization & Registration
//
// An application using galog will follow the pattern of setting up basic
// parameters and registering backends before starting to use it, such as:
//
//	galog.SetMinVerbosity(2)
//	galog.SetLevel(galog.DebugLevel)
//
//	ctx := context.Background()
//	galog.RegisterBackend(ctx, galog.NewFileBackend("/var/log/app.log"))
//	galog.Info("Logger initialized.")
//	...
//
// Multiple backends can be registered at the same time and a backend can be
// unregistered with [UnregisterBackend] at any time.
//
// # Verbosity
//
// In addition to log levels ([ErrorLevel], [WarningLevel], [InfoLevel] and
// [DebugLevel]) galog also provides a notion of verbosity. An application may
// have for example a definition of multiple verbosity level logs by using [V]
// such as:
//
//	// Will be logged if the set verbosity is greater or equal than 1.
//	galog.V(1).Info("Info logged message with verbosity 1")
//	// Will be logged if the set verbosity is greater or equal than 2.
//	galog.V(2).Info("Info logged message with verbosity 2")
//
// The application can then define its current verbosity by using
// [SetMinVerbosity]. Combined with [SetLevel] the application can fine tune
// levels of interest and its verbosity.
//
// # Enqueued Log Entries
//
// Internally galog implements a queueing strategy when backends fail to write
// the log entries to the backing storage.
//
// When a backend fails to write an entry the [Backend]'s Log() function returns
// an error reporting the failure, the enqueuing code will detect such error
// condition and append the failed entry to a dedicated queue (each backend has
// its own queue).
//
// Honoring the frequency set with [QueueRetryFrequency] galog will keep
// retrying to write entries from the queue in such a defined frequency. All log
// entries will be queued if there are pending writes (so the order of writes
// reflect the same order the entries were created), otherwise the entries are
// written immediately.
//
// When the queue size limit is reached galog will start to drop the oldest
// entries so the newest ones have room to be enqueued.
//
// Each backend has its own definition of "good default" queue sizes, however
// applications can redefine them by using SetQueueSize() from [Config] -
// accessing it with [Backend]'s Config() operation, i.e.:
//
//	ctx := context.Background()
//
//	fileBackend := galog.NewFileBackend("/var/log/app.log")
//	fileBackend.Config().SetQueueSize(200) // resetting the FileBackend's queue size to 200
//
//	galog.RegisterBackend(ctx, fileBackend)
//	galog.Info("Logger initialized.")
//	...
//
// # Shutting down
//
// To nicely shutdown galog exposes the [Shutdown] function that will attempt
// to write all the pending entries from all backends and force a flush (see
// [Backend]'s Flush() function). When completed all previously registered
// backends will also be unregistered.
package galog
