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
	"encoding/json"
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
	values       []interface{}
	jsonFormat   bool
}

// NewLogger creates a new logr.Logger that writes to stdout.
func NewLogger() logr.Logger {
	stdErrLogger := log.New(os.Stderr, "", 0)

	value, isSet := os.LookupEnv(enableCloudLoggingEnvVar)
	var enableCloudLogging bool

	if !isSet {
		stdErrLogger.Printf("[INFO] Environment variable %s is unset; logs will be emitted to stdout and stderr", enableCloudLoggingEnvVar)
		enableCloudLogging = false
	} else {
		var err error
		enableCloudLogging, err = strconv.ParseBool(value)
		if err != nil {
			stdErrLogger.Printf("[ERROR] Failed to parse %s='%s' to bool; logs will be emitted to stdout and stderr", enableCloudLoggingEnvVar, value)
			enableCloudLogging = false
		}
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
				stdErrLogger.Printf("[ERROR] Failed to initialize Google Cloud Logging client: %v", err)
			}
		} else {
			stdErrLogger.Printf("[ERROR] Failed to get project ID: %v", err)
		}
	}

	return logr.New(logSink{
		stdOutLogger: log.New(os.Stdout, "", 0),
		stdErrLogger: stdErrLogger,
		cloudLogger:  cloudLogger,
		jsonFormat:   strings.ToLower(os.Getenv("LOG_FORMAT")) == "json",
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
	if ls.jsonFormat {
		payload := make(map[string]interface{})
		payload["severity"] = "INFO"
		payload["component"] = "metric-provider"
		payload["message"] = msg
		populateKVs(payload, kvs)
		if jsonBytes, err := json.Marshal(payload); err == nil {
			ls.stdOutLogger.Println(string(jsonBytes))
		}
	} else {
		ls.stdOutLogger.Printf("[INFO] [METRIC-PROVIDER] %s %s\n", msg, ls.formatKVs(kvs))
	}
}

func (ls logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	kvs := append(ls.values, keysAndValues...)
	fullMsg := fmt.Sprintf("%s: %v", msg, err)
	formattedMsg := fmt.Sprintf("[ERROR] [METRIC-PROVIDER] %s %s\n", fullMsg, ls.formatKVs(kvs))

	if ls.cloudLogger != nil {
		payload := make(map[string]interface{})
		payload["component"] = "metric-provider"
		payload["message"] = fullMsg
		populateKVs(payload, kvs)

		ls.cloudLogger.Log(logging.Entry{
			Payload:  payload,
			Severity: logging.Error,
		})
	} else {
		if ls.jsonFormat {
			payload := make(map[string]interface{})
			payload["severity"] = "ERROR"
			payload["component"] = "metric-provider"
			payload["message"] = fullMsg
			populateKVs(payload, kvs)
			if jsonBytes, err := json.Marshal(payload); err == nil {
				ls.stdErrLogger.Println(string(jsonBytes))
			}
		} else {
			ls.stdErrLogger.Print(formattedMsg)
		}
	}
}

func (ls logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	newLogger := ls
	newLogger.values = append(newLogger.values, keysAndValues...)
	return newLogger
}

func (ls logSink) WithName(name string) logr.LogSink {
	return ls
}

func (ls logSink) formatKVs(keysAndValues []interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(" ")
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			fmt.Fprintf(&sb, "%v=%+v", keysAndValues[i], keysAndValues[i+1])
		} else {
			fmt.Fprintf(&sb, "%v=", keysAndValues[i])
		}
		if i+2 < len(keysAndValues) {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func populateKVs(payload map[string]interface{}, kvs []interface{}) {
	for i := 0; i < len(kvs); i += 2 {
		if i+1 < len(kvs) {
			keyStr, ok := kvs[i].(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", kvs[i])
			}
			payload[keyStr] = kvs[i+1]
		}
	}
}
