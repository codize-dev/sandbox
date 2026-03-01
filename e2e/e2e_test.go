//go:build e2e

package e2e

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

//go:embed tests/*.yml
var testFiles embed.FS

const serverURL = "http://localhost:8080"

var runtimeEntryFile = map[string]string{
	"node": "index.js",
	"ruby": "main.rb",
}

type testFile struct {
	Tests []testCase `yaml:"tests"`
}

type testCase struct {
	Name   string     `yaml:"name"`
	Input  testInput  `yaml:"input"`
	Output testOutput `yaml:"output"`
}

type testInput struct {
	Runtime string `yaml:"runtime"`
	Code    string `yaml:"code"`
}

type testOutput struct {
	Status int       `yaml:"status"`
	Run    runOutput `yaml:"run"`
}

type runOutput struct {
	Stdout   string `yaml:"stdout"`
	Stderr   string `yaml:"stderr"`
	Output   string `yaml:"output"`
	ExitCode int    `yaml:"exit_code"`
	Status   string `yaml:"status"`
}

type apiRequest struct {
	Runtime string    `json:"runtime"`
	Files   []apiFile `json:"files"`
}

type apiFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type apiResponse struct {
	Run apiRunResult `json:"run"`
}

type apiRunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Status   string `json:"status"`
}

func decodeBase64(t *testing.T, encoded, field string) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err, "failed to decode base64 for %s: raw value was %q", field, encoded)
	return string(decoded)
}

func TestE2E(t *testing.T) {
	files, err := fs.Glob(testFiles, "tests/*.yml")
	require.NoError(t, err, "failed to glob test files")
	require.NotEmpty(t, files, "no test files found")

	for _, file := range files {
		data, err := fs.ReadFile(testFiles, file)
		require.NoError(t, err, "failed to read test file: %s", file)

		var tf testFile
		err = yaml.Unmarshal(data, &tf)
		require.NoError(t, err, "failed to parse test file: %s", file)

		fileName := strings.TrimSuffix(path.Base(file), ".yml")
		require.NotEmpty(t, tf.Tests, "test file %s contains no test cases", file)

		for i, tc := range tf.Tests {
			t.Run(fmt.Sprintf("%s/%d/%s", fileName, i, tc.Name), func(t *testing.T) {
				t.Parallel()
				entryFile, ok := runtimeEntryFile[tc.Input.Runtime]
				require.True(t, ok, "unknown runtime: %s", tc.Input.Runtime)

				reqBody := apiRequest{
					Runtime: tc.Input.Runtime,
					Files: []apiFile{
						{
							Name:    entryFile,
							Content: base64.StdEncoding.EncodeToString([]byte(tc.Input.Code)),
						},
					},
				}

				bodyBytes, err := json.Marshal(reqBody)
				require.NoError(t, err, "failed to marshal request body")

				url := fmt.Sprintf("%s/v1/run", serverURL)
				resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
				require.NoError(t, err, "failed to send request to %s", url)
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, tc.Output.Status, resp.StatusCode, "unexpected HTTP status code")

				var apiResp apiResponse
				err = json.NewDecoder(resp.Body).Decode(&apiResp)
				require.NoError(t, err, "failed to decode response body")

				actualStdout := decodeBase64(t, apiResp.Run.Stdout, "stdout")
				actualStderr := decodeBase64(t, apiResp.Run.Stderr, "stderr")
				actualOutput := decodeBase64(t, apiResp.Run.Output, "output")

				assert.Equal(t, tc.Output.Run.Stdout, actualStdout, "stdout mismatch")
				assert.Equal(t, tc.Output.Run.Stderr, actualStderr, "stderr mismatch")
				assert.Equal(t, tc.Output.Run.Output, actualOutput, "output mismatch")
				assert.Equal(t, tc.Output.Run.ExitCode, apiResp.Run.ExitCode, "exit_code mismatch")
				assert.Equal(t, tc.Output.Run.Status, apiResp.Run.Status, "status mismatch")
			})
		}
	}
}
