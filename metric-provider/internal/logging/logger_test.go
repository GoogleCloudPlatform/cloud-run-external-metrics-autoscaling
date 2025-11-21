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
	"testing"
)

func TestFormatKVs(t *testing.T) {
	tests := []struct {
		name string
		kvs  []interface{}
		want string
	}{
		{
			name: "empty kvs",
			kvs:  []interface{}{},
			want: "",
		},
		{
			name: "single key value pair",
			kvs:  []interface{}{"key1", "value1"},
			want: " key1=value1",
		},
		{
			name: "multiple key value pairs",
			kvs:  []interface{}{"key1", "value1", "key2", 123},
			want: " key1=value1 key2=123",
		},
		{
			name: "key without value",
			kvs:  []interface{}{"key1"},
			want: " key1=",
		},
		{
			name: "key with struct value",
			kvs:  []interface{}{"key1", struct{ A string }{A: "test"}},
			want: " key1={A:test}",
		},
		{
			name: "key with nil value",
			kvs:  []interface{}{"key1", nil},
			want: " key1=<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := logSink{}
			if got := ls.formatKVs(tt.kvs); got != tt.want {
				t.Errorf("formatKVs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithName(t *testing.T) {
	tests := []struct {
		name           string
		initialPrefix  string
		withName       string
		expectedPrefix string
	}{
		{
			name:           "empty initial prefix",
			initialPrefix:  "",
			withName:       "test",
			expectedPrefix: "[test]",
		},
		{
			name:           "existing initial prefix",
			initialPrefix:  "[METRIC-PROVIDER]",
			withName:       "test",
			expectedPrefix: "[METRIC-PROVIDER/test]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := logSink{prefix: tt.initialPrefix}
			newSink := ls.WithName(tt.withName).(logSink)
			if newSink.prefix != tt.expectedPrefix {
				t.Errorf("WithName() prefix = %q, want %q", newSink.prefix, tt.expectedPrefix)
			}
		})
	}
}
