// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"crema/metric-provider/internal/clients"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/go-logr/logr"
)

const enableCloudLoggingEnvVar = "ENABLE_CLOUD_LOGGING"

// logSink implements logr.LogSink.
type logSink struct {
	stdOutLogger *log.Logger
	stdErrLogger *log.Logger
	cloudLogger  *logging.Logger
	prefix       string
	values       []interface{}
}

// NewLogger creates a new logr.Logger that writes to stdout.
func NewLogger() logr.Logger {
	prefix := "[METRIC-PROVIDER]"
	stdErrLogger := log.New(os.Stderr, "", 0)

	enableCloudLogging, err := strconv.ParseBool(os.Getenv(enableCloudLoggingEnvVar))
	if err != nil {
		stdErrLogger.Printf("[ERROR] %s Failed to parse %s to bool; logs will be mitted to stdout and stderr", prefix, enableCloudLoggingEnvVar)
	}

	var cloudLogger *logging.Logger

	if enableCloudLogging {
		cloudRunMetadataClient := clients.CloudRunMetadata()
		projectID, err := cloudRunMetadataClient.GetProjectID()
		if err == nil {
			ctx := context.Background()
			client, err := logging.NewClient(ctx, projectID)
			if err == nil {
				cloudLogger = client.Logger("crema")
			} else {
				stdErrLogger.Printf("[ERROR] %s Failed to initialize Google Cloud Logging client: %v", prefix, err)
			}
		} else {
			stdErrLogger.Printf("[ERROR] %s Failed to get project ID: %v", prefix, err)
		}
	}

	return logr.New(logSink{
		stdOutLogger: log.New(os.Stdout, "", 0),
		stdErrLogger: stdErrLogger,
		cloudLogger:  cloudLogger,
		prefix:       prefix,
	})
}

// Required for logr interface
func (ls logSink) Init(info logr.RuntimeInfo) {
}

// Required for logr interface
func (ls logSink) Enabled(level int) bool {
	return true
}

func (ls logSink) Info(level int, msg string, keysAndValues ...interface{}) {
	kvs := append(ls.values, keysAndValues...)
	ls.stdOutLogger.Printf("[INFO] %s %s %s", ls.prefix, msg, ls.formatKVs(kvs))
}

func (ls logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	kvs := append(ls.values, keysAndValues...)
	msg = fmt.Sprintf("[ERROR] %s %s: %v%s", ls.prefix, msg, err, ls.formatKVs(kvs))

	if ls.cloudLogger != nil {
		payload := make(map[string]interface{})
		payload["message"] = msg

		// Include the key-values in the payload for searchability
		for i := 0; i < len(kvs); i += 2 {
			if i+1 < len(kvs) {
				payload[fmt.Sprintf("%v", kvs[i])] = kvs[i+1]
			}
		}

		ls.cloudLogger.Log(logging.Entry{
			Payload:  payload,
			Severity: logging.Error,
		})
	} else {
		ls.stdErrLogger.Print(msg)
	}
}

func (ls logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	newLogger := ls
	newLogger.values = append(newLogger.values, keysAndValues...)
	return newLogger
}

func (ls logSink) WithName(name string) logr.LogSink {
	newLogger := ls
	if len(ls.prefix) > 0 {
		// Remove brackets
		prefix := ls.prefix[1 : len(ls.prefix)-1]
		newLogger.prefix = fmt.Sprintf("[%s/%s]", prefix, name)
	} else {
		newLogger.prefix = fmt.Sprintf("[%s]", name)
	}
	return newLogger
}

func (ls logSink) formatKVs(keysAndValues []interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(" ")
	for i := 0; i < len(keysAndValues); i += 2 {
		sb.WriteString(fmt.Sprintf("%s=", keysAndValues[i]))
		if i+1 < len(keysAndValues) {
			sb.WriteString(fmt.Sprintf("%+v", keysAndValues[i+1]))
		}
		if i+2 < len(keysAndValues) {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}
