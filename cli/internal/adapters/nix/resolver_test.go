package nix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/nix"
	"go.trai.ch/same/internal/core/domain"
)

// MockRoundTripper is a helper to mock http.Client behavior.
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) *http.Response
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req), nil
}

func newMockClient(handler func(req *http.Request) *http.Response) *http.Client {
	return &http.Client{
		Transport: &MockRoundTripper{RoundTripFunc: handler},
	}
}

func TestResolver_Resolve(t *testing.T) {
	// Create a temporary cache directory
	tmpDir := t.TempDir()

	t.Run("Success", func(t *testing.T) {
		// Mock response from NixHub
		mockResp := nix.NixHubResponse{
			Systems: map[string]nix.SystemResponse{
				"x86_64-darwin": {
					FlakeInstallable: nix.FlakeInstallable{
						Ref:      nix.FlakeRef{Rev: "commit123"},
						AttrPath: "legacyPackages.x86_64-darwin.go_1_21",
					},
				},
				"aarch64-darwin": {
					FlakeInstallable: nix.FlakeInstallable{
						Ref:      nix.FlakeRef{Rev: "commit123"},
						AttrPath: "legacyPackages.aarch64-darwin.go_1_21",
					},
				},
				"x86_64-linux": {
					FlakeInstallable: nix.FlakeInstallable{
						Ref:      nix.FlakeRef{Rev: "commit123"},
						AttrPath: "legacyPackages.x86_64-linux.go_1_21",
					},
				},
				"aarch64-linux": {
					FlakeInstallable: nix.FlakeInstallable{
						Ref:      nix.FlakeRef{Rev: "commit123"},
						AttrPath: "legacyPackages.aarch64-linux.go_1_21",
					},
				},
			},
		}

		respBody, _ := json.Marshal(mockResp)

		client := newMockClient(func(req *http.Request) *http.Response {
			if req.URL.String() == "https://search.devbox.sh/v2/resolve?name=go&version=1.21.0" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(respBody)),
					Header:     make(http.Header),
				}
			}
			return &http.Response{StatusCode: http.StatusNotFound}
		})

		resolver, err := nix.NewResolverWithClient(tmpDir, client)
		require.NoError(t, err)

		commit, attr, err := resolver.Resolve(context.Background(), "go", "1.21.0")
		require.NoError(t, err)
		assert.Equal(t, "commit123", commit)

		// Expected attribute path depends on the current system
		sys := nix.GetCurrentSystem()
		expectedAttr := "legacyPackages." + sys + ".go_1_21"
		assert.Equal(t, expectedAttr, attr)
	})

	t.Run("NotFound", func(t *testing.T) {
		client := newMockClient(func(_ *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("")),
			}
		})

		resolver, err := nix.NewResolverWithClient(filepath.Join(tmpDir, "404"), client)
		require.NoError(t, err)

		_, _, err = resolver.Resolve(context.Background(), "unknown", "1.0")
		// Use string check for robustness if ErrorIs fails with zerr wrapping
		require.Error(t, err)
		assert.Contains(t, err.Error(), domain.ErrNixPackageNotFound.Error())
	})

	t.Run("APIError", func(t *testing.T) {
		client := newMockClient(func(_ *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
			}
		})

		resolver, err := nix.NewResolverWithClient(filepath.Join(tmpDir, "500"), client)
		require.NoError(t, err)

		_, _, err = resolver.Resolve(context.Background(), "go", "1.21.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), domain.ErrNixAPIRequestFailed.Error())
	})

	t.Run("CacheHit", func(t *testing.T) {
		// Prepare cache manually by running a success case first
		cacheDir := filepath.Join(tmpDir, "cache_hit")

		setupClient := newMockClient(func(_ *http.Request) *http.Response {
			mockResp := nix.NixHubResponse{
				Systems: map[string]nix.SystemResponse{
					nix.GetCurrentSystem(): {
						FlakeInstallable: nix.FlakeInstallable{
							Ref:      nix.FlakeRef{Rev: "cached_commit"},
							AttrPath: "cached_attr",
						},
					},
				},
			}
			body, _ := json.Marshal(mockResp)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(body)),
			}
		})

		rSetup, err := nix.NewResolverWithClient(cacheDir, setupClient)
		require.NoError(t, err)
		_, _, err = rSetup.Resolve(context.Background(), "cached_tool", "1.0")
		require.NoError(t, err)

		// Now use a panic client to ensure no API call is made
		panicClient := newMockClient(func(_ *http.Request) *http.Response {
			panic("HTTP client should not be called on cache hit")
		})

		rTest, err := nix.NewResolverWithClient(cacheDir, panicClient)
		require.NoError(t, err)

		commit, attr, err := rTest.Resolve(context.Background(), "cached_tool", "1.0")
		require.NoError(t, err)
		assert.Equal(t, "cached_commit", commit)
		assert.Equal(t, "cached_attr", attr)
	})
}
