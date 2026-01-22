package nix_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/nix"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestParseNixDevEnv(t *testing.T) {
	// Sample JSON output from nix print-dev-env
	sampleJSON := []byte(`{
		"variables": {
			"PATH": {
				"type": "array",
				"value": ["/nix/store/abc-go/bin", "/nix/store/xyz-git/bin"]
			},
			"GOROOT": {
				"type": "string",
				"value": "/nix/store/abc-go/share/go"
			},
			"TERM": {
				"type": "string",
				"value": "xterm-256color"
			},
			"UNKNOWN_VAR": {
				"type": "unknown_type",
				"value": 123
			}
		}
	}`)

	env, err := nix.ParseNixDevEnv(sampleJSON)
	require.NoError(t, err)

	// Verify allowed variables are present and formatted correctly
	assert.Contains(t, env, "PATH=/nix/store/abc-go/bin:/nix/store/xyz-git/bin")
	assert.Contains(t, env, "GOROOT=/nix/store/abc-go/share/go")

	// Verify blacklisted variables (TERM) are excluded
	for _, e := range env {
		assert.NotContains(t, e, "TERM=")
	}

	// Verify unknown types are skipped
	for _, e := range env {
		assert.NotContains(t, e, "UNKNOWN_VAR=")
	}
}

func TestEnvFactory_GetEnvironment_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Strict mock: EXPECT NO CALLS to Resolve
	mockResolver := mocks.NewMockDependencyResolver(ctrl)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)

	tools := map[string]string{
		"go": "go@1.21.0",
	}

	// 1. Manually populate cache
	envID := domain.GenerateEnvID(tools)
	cachePath := filepath.Join(tmpDir, "environments", envID+".json")

	// Create cache contents
	cachedEnv := []string{"CACHED_VAR=123", "GOTOOLCHAIN=local"}
	data, err := json.Marshal(cachedEnv)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), domain.DirPerm))
	require.NoError(t, os.WriteFile(cachePath, data, domain.PrivateFilePerm))

	// 2. Call GetEnvironment
	env, err := factory.GetEnvironment(context.Background(), tools)
	require.NoError(t, err)

	// 3. Verify returned environment matches cache (plus standard injections)
	assert.Contains(t, env, "CACHED_VAR=123")
	assert.Contains(t, env, "GOTOOLCHAIN=local")
	// Check injection of TMPDIR
	foundTmp := false
	for _, e := range env {
		if len(e) >= 7 && e[:7] == "TMPDIR=" {
			foundTmp = true
			break
		}
	}
	assert.True(t, foundTmp, "TMPDIR should be injected")
}

func TestEnvFactory_GetEnvironment_ResolutionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockResolver := mocks.NewMockDependencyResolver(ctrl)
	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)

	tools := map[string]string{
		"go": "go@1.21.0",
	}

	// Expect Resolve call and return error
	mockResolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.21.0").
		Return("", "", assert.AnError)

	env, err := factory.GetEnvironment(context.Background(), tools)
	require.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "failed to resolve tool")
}

func TestSaveEnvToCache_PermissionDenied(t *testing.T) {
	// Create a read-only directory
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "env.json")

	// Make dir read-only
	//nolint:gosec // Directory permissions need to be read-only for verification
	require.NoError(t, os.Chmod(tmpDir, 0o500))
	defer func() {
		//nolint:gosec // Restore permissions for cleanup
		_ = os.Chmod(tmpDir, domain.DirPerm)
	}()

	env := []string{"VAR=val"}
	err := nix.SaveEnvToCache(cachePath, env)

	// Should return error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create temp cache file")
}

func TestParseNixDevEnv_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{ invalid json `)
	_, err := nix.ParseNixDevEnv(invalidJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal nix output")
}

// mockExecCommand mocks exec.CommandContext for testing.
// It effectively replaces the command with a call to the test binary itself,
// invoking TestHelperProcess.
func mockExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	//nolint:gosec // Test helper calls
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	// Pass through environment variables to control behavior
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is the fake command execution handler.
func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]

	// Handle 'nix print-dev-env'
	if cmd == "nix" && len(args) > 0 && args[0] == "print-dev-env" {
		// Output a valid dummy JSON environment
		_, _ = fmt.Fprint(os.Stdout, `{"variables": {"TEST_VAR": {"type": "string", "value": "test_value"}}}`)
		os.Exit(0)
	}
	os.Exit(0)
}

func TestEnvFactory_GetEnvironment_FastPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResolver := mocks.NewMockDependencyResolver(ctrl)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)

	tools := map[string]string{"go": "go@1.22.0"}
	envID := domain.GenerateEnvID(tools)

	// Pre-populate cache
	cacheDir := filepath.Join(tmpDir, "environments")
	require.NoError(t, os.MkdirAll(cacheDir, domain.DirPerm))
	cachePath := filepath.Join(cacheDir, envID+".json")

	cachedEnv := []string{"CACHED=true"}
	data, err := json.Marshal(cachedEnv)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, domain.PrivateFilePerm))

	// Mock Resolver should NOT be called (Fast Path)
	// No EXPECT() on mockResolver implies any call will fail the test.

	env, err := factory.GetEnvironment(context.Background(), tools)
	require.NoError(t, err)
	assert.Contains(t, env, "CACHED=true")
}

func TestEnvFactory_GetEnvironment_CacheCorruption(t *testing.T) {
	// Setup exec mock
	restore := nix.SetExecCommandContext(mockExecCommand)
	defer restore()

	ctrl := gomock.NewController(t)
	mockResolver := mocks.NewMockDependencyResolver(ctrl)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)

	tools := map[string]string{"go": "go@1.22.0"}
	envID := domain.GenerateEnvID(tools)

	// Pre-populate corrupt cache
	cacheDir := filepath.Join(tmpDir, "environments")
	require.NoError(t, os.MkdirAll(cacheDir, domain.DirPerm))
	cachePath := filepath.Join(cacheDir, envID+".json")
	require.NoError(t, os.WriteFile(cachePath, []byte("invalid json"), domain.PrivateFilePerm))

	// Expect proper resolution fallback
	mockResolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.22.0").
		Return("commit_hash", "packages.go", nil)

	env, err := factory.GetEnvironment(context.Background(), tools)
	require.NoError(t, err)
	// Verify it came from the helper process (fresh build)
	assert.Contains(t, env, "TEST_VAR=test_value")
}

func TestEnvFactory_GetEnvironment_CacheWriteFailure(t *testing.T) {
	// Setup exec mock
	restore := nix.SetExecCommandContext(mockExecCommand)
	defer restore()

	ctrl := gomock.NewController(t)
	mockResolver := mocks.NewMockDependencyResolver(ctrl)

	tmpDir := t.TempDir()

	// Create the environments directory and make it read-only to force write failure
	envDir := filepath.Join(tmpDir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o500)) // Read-only

	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)
	tools := map[string]string{"go": "go@1.22.0"}

	// Expect resolution
	mockResolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.22.0").
		Return("commit_hash", "packages.go", nil)

	// Should succeed despite cache write failure
	env, err := factory.GetEnvironment(context.Background(), tools)
	require.NoError(t, err)
	assert.Contains(t, env, "TEST_VAR=test_value")
}

func TestResolver_Resolve_EdgeCases(t *testing.T) {
	// Helper to create a test server
	newServer := func(handler http.HandlerFunc) *httptest.Server {
		return httptest.NewServer(handler)
	}

	t.Run("API Failure", func(t *testing.T) {
		server := newServer(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		defer server.Close()

		// Hijack the API base URL by creating a resolver that points to our server?
		// nix.Resolver uses a constant `nixHubAPIBase`.
		// We CANNOT change the constant.
		// However, we can use a custom Transport in the client to intercept requests to that host.
		// Detailed implementation below.

		// Wait, we can't easily change the URL.
		// Better approach: Use http.RoundTripper interception.

		transport := &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v2/resolve" {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       http.NoBody,
						Header:     make(http.Header),
					}, nil
				}
				return nil, fmt.Errorf("unexpected request")
			},
		}
		client := &http.Client{Transport: transport}

		resolver, err := nix.NewResolverWithClient(t.TempDir(), client)
		require.NoError(t, err)

		_, _, err = resolver.Resolve(context.Background(), "go", "1.22.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), domain.ErrNixAPIRequestFailed.Error())
	})

	t.Run("Unsupported System", func(t *testing.T) {
		transport := &testTransport{
			handler: func(_ *http.Request) (*http.Response, error) {
				// Return valid JSON but WITHOUT the current system
				// We assume current system is NOT "fake-system"
				resp := `{"systems": {"fake-system": {}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(resp)),
					Header:     make(http.Header),
				}, nil
			},
		}
		resolver, err := nix.NewResolverWithClient(t.TempDir(), &http.Client{Transport: transport})
		require.NoError(t, err)

		_, _, err = resolver.Resolve(context.Background(), "go", "1.22.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), domain.ErrNixPackageNotFound.Error())
	})

	t.Run("Atomic Write Failure", func(t *testing.T) {
		// Create a cache usage scenario
		tmpDir := t.TempDir()
		// Make the cache directory read-only (0500)
		//nolint:gosec // Test permissions
		require.NoError(t, os.Chmod(tmpDir, 0o500))
		defer func() {
			//nolint:gosec // Cleanup
			_ = os.Chmod(tmpDir, domain.DirPerm)
		}()

		// Setup client to return success
		transport := &testTransport{
			handler: func(_ *http.Request) (*http.Response, error) {
				system := nix.GetCurrentSystem()
				// Valid response
				resp := fmt.Sprintf(`{
					"systems": {
						"%s": {
							"flake_installable": {
								"ref": {"rev": "commit_hash"},
								"attr_path": "attr_path"
							}
						}
					}
				}`, system)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(resp)),
					Header:     make(http.Header),
				}, nil
			},
		}

		resolver, err := nix.NewResolverWithClient(tmpDir, &http.Client{Transport: transport})
		require.NoError(t, err)

		// Should succeed despite failing to write cache
		hash, path, err := resolver.Resolve(context.Background(), "go", "1.22.0")
		require.NoError(t, err)
		assert.Equal(t, "commit_hash", hash)
		assert.Equal(t, "attr_path", path)
	})
}

// testTransport implements http.RoundTripper for testing.
type testTransport struct {
	handler func(*http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.handler(req)
}
