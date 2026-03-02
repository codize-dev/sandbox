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

type testFile struct {
	Tests []testCase `yaml:"tests"`
}

type testCase struct {
	Name   string     `yaml:"name"`
	Input  testInput  `yaml:"input"`
	Output testOutput `yaml:"output"`
}

type testInputFile struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type testInput struct {
	Runtime string          `yaml:"runtime"`
	Files   []testInputFile `yaml:"files"`
}

type testOutput struct {
	Status  int        `yaml:"status"`
	Compile *runOutput `yaml:"compile"`
	Run     *runOutput `yaml:"run"`
	Error   string     `yaml:"error"`
}

type runOutput struct {
	Stdout   string  `yaml:"stdout"`
	Stderr   string  `yaml:"stderr"`
	Output   string  `yaml:"output"`
	ExitCode int     `yaml:"exit_code"`
	Status   string  `yaml:"status"`
	Signal   *string `yaml:"signal"`
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
	Compile *apiRunResult `json:"compile"`
	Run     *apiRunResult `json:"run"`
}

type apiRunResult struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Output   string  `json:"output"`
	ExitCode int     `json:"exit_code"`
	Status   string  `json:"status"`
	Signal   *string `json:"signal"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
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
				files := make([]apiFile, len(tc.Input.Files))
				for i, f := range tc.Input.Files {
					files[i] = apiFile{
						Name:    f.Name,
						Content: base64.StdEncoding.EncodeToString([]byte(f.Content)),
					}
				}

				reqBody := apiRequest{
					Runtime: tc.Input.Runtime,
					Files:   files,
				}

				bodyBytes, err := json.Marshal(reqBody)
				require.NoError(t, err, "failed to marshal request body")

				url := fmt.Sprintf("%s/v1/run", serverURL)
				resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
				require.NoError(t, err, "failed to send request to %s", url)
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, tc.Output.Status, resp.StatusCode, "unexpected HTTP status code")

				if tc.Output.Error != "" {
					var errResp apiErrorResponse
					err = json.NewDecoder(resp.Body).Decode(&errResp)
					require.NoError(t, err, "failed to decode error response body")

					assert.Equal(t, tc.Output.Error, errResp.Error, "error message mismatch")
				} else {
					var apiResp apiResponse
					err = json.NewDecoder(resp.Body).Decode(&apiResp)
					require.NoError(t, err, "failed to decode response body")

					if tc.Output.Compile == nil {
						assert.Nil(t, apiResp.Compile, "compile should be null")
					} else {
						require.NotNil(t, apiResp.Compile, "compile should not be null")
						assert.Equal(t, tc.Output.Compile.Stdout, decodeBase64(t, apiResp.Compile.Stdout, "compile stdout"), "compile stdout mismatch")
						assert.Equal(t, tc.Output.Compile.Stderr, decodeBase64(t, apiResp.Compile.Stderr, "compile stderr"), "compile stderr mismatch")
						assert.Equal(t, tc.Output.Compile.Output, decodeBase64(t, apiResp.Compile.Output, "compile output"), "compile output mismatch")
						assert.Equal(t, tc.Output.Compile.ExitCode, apiResp.Compile.ExitCode, "compile exit_code mismatch")
						assert.Equal(t, tc.Output.Compile.Status, apiResp.Compile.Status, "compile status mismatch")
						assert.Equal(t, tc.Output.Compile.Signal, apiResp.Compile.Signal, "compile signal mismatch")
					}

					if tc.Output.Run == nil {
						assert.Nil(t, apiResp.Run, "run should be null")
					} else {
						require.NotNil(t, apiResp.Run, "run should not be null")
						assert.Equal(t, tc.Output.Run.Stdout, decodeBase64(t, apiResp.Run.Stdout, "run stdout"), "run stdout mismatch")
						assert.Equal(t, tc.Output.Run.Stderr, decodeBase64(t, apiResp.Run.Stderr, "run stderr"), "run stderr mismatch")
						assert.Equal(t, tc.Output.Run.Output, decodeBase64(t, apiResp.Run.Output, "run output"), "run output mismatch")
						assert.Equal(t, tc.Output.Run.ExitCode, apiResp.Run.ExitCode, "run exit_code mismatch")
						assert.Equal(t, tc.Output.Run.Status, apiResp.Run.Status, "run status mismatch")
						assert.Equal(t, tc.Output.Run.Signal, apiResp.Run.Signal, "run signal mismatch")
					}
				}
			})
		}
	}
}
