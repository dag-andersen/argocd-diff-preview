package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_JSON(t *testing.T) {
	for _, debug := range []bool{false, true} {
		var buf bytes.Buffer
		logger := newLogger(&Config{LogFormat: "json", Debug: debug}, &buf)
		logger.Error().Str("app", "my-app").Msg("failed to render")

		var entry map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry), "output is not JSON: %q", buf.String())
		assert.Equal(t, "error", entry["level"])
		assert.Equal(t, "failed to render", entry["message"])
		assert.Equal(t, "my-app", entry["app"])
		assert.Contains(t, entry, "time")
	}
}

func TestNewLogger_Human(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&Config{LogFormat: "human"}, &buf)
	logger.Error().Str("app", "my-app").Msg("failed to render")

	assert.Equal(t, "failed to render (app: my-app)", strings.TrimSpace(buf.String()))
}
