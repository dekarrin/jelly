package logging

import (
	"path/filepath"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	testCases := []struct {
		name       string
		provider   jelly.LogProvider
		filename   string
		expectType jelly.Logger
		expectErr  bool
	}{
		{
			name:       "jellog log",
			provider:   jelly.Jellog,
			filename:   "test-jellog.log",
			expectType: jellogLogger{},
		},
		{
			name:       "standard log",
			provider:   jelly.StdLog,
			filename:   "test-std.log",
			expectType: stdLogger{},
		},
		{
			name:      "NoLog provider is an error",
			provider:  jelly.NoLog,
			filename:  "test-none.log",
			expectErr: true,
		},
		{
			name:      "unknown provider is an error",
			provider:  jelly.LogProvider(-1),
			filename:  "test-unknown.log",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, tc.filename)

			actual, err := New(tc.provider, filePath)

			if tc.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				assert.IsType(tc.expectType, actual)
			}
		})
	}
}
