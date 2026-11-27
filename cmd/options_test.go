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

func TestRawOptionsToConfig_IncludeHelmHooksRenderMethod(t *testing.T) {
	baseOptions := func(renderMethod string) RawOptions {
		return RawOptions{
			BaseBranch:       "main",
			TargetBranch:     "feature",
			Repo:             "example/repo",
			RenderMethod:     renderMethod,
			IncludeHelmHooks: true,
			DryRun:           true,
		}
	}

	t.Run("rejects CLI renderer", func(t *testing.T) {
		options := baseOptions("cli")
		_, err := options.ToConfig()
		require.EqualError(t, err, "--include-helm-hooks is incompatible with --render-method=cli; use --render-method=server-api or --render-method=repo-server-api")
	})

	for _, renderMethod := range []string{"server-api", "repo-server-api"} {
		t.Run(renderMethod, func(t *testing.T) {
			options := baseOptions(renderMethod)
			cfg, err := options.ToConfig()
			require.NoError(t, err)
			assert.True(t, cfg.IncludeHelmHooks)
			assert.Equal(t, RenderMethod(renderMethod), cfg.RenderMethod)
		})
	}
}

func TestRawOptionsToConfig_IgnoreArrayOrder(t *testing.T) {
	baseOptions := func(ignoreArrayOrder string) RawOptions {
		return RawOptions{
			BaseBranch:       "main",
			TargetBranch:     "feature",
			Repo:             "example/repo",
			RenderMethod:     "cli",
			DryRun:           true,
			IgnoreArrayOrder: ignoreArrayOrder,
		}
	}

	t.Run("empty", func(t *testing.T) {
		options := baseOptions("")
		cfg, err := options.ToConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.IgnoreArrayOrder)
	})

	t.Run("parses list of paths", func(t *testing.T) {
		options := baseOptions(".spec.template.spec.containers[*].env, ..volumeMounts\n{.spec.template.spec.volumes}")
		cfg, err := options.ToConfig()
		require.NoError(t, err)
		var paths []string
		for _, rule := range cfg.IgnoreArrayOrder {
			paths = append(paths, rule.String())
		}
		assert.Equal(t, []string{".spec.template.spec.containers[*].env", "..volumeMounts", "{.spec.template.spec.volumes}"}, paths)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		options := baseOptions(".spec.containers[?(@.name=='app')].env")
		_, err := options.ToConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ignore-array-order")
	})
}
