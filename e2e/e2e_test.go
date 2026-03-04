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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

//go:embed tests
var testFiles embed.FS

const serverURL = "http://localhost:8080"

type testFile struct {
	Tests []testCase `yaml:"tests"`
}

type fileType string

const (
	fileTypeRaw  fileType = "raw"
	fileTypeFill fileType = "fill"
)

type testInputFile struct {
	Name    string   `yaml:"name"`
	Type    fileType `yaml:"type"`
	Content string   `yaml:"content"`
	Size    int      `yaml:"size"`
}

func (f testInputFile) resolveContent() (string, error) {
	switch f.Type {
	case fileTypeRaw, "":
		return f.Content, nil
	case fileTypeFill:
		return strings.Repeat("A", f.Size), nil
	default:
		return "", fmt.Errorf("unknown file type: %q", f.Type)
	}
}

type testInput struct {
	Runtime string          `yaml:"runtime"`
	Files   []testInputFile `yaml:"files"`
}

type testOutput struct {
	Status int            `yaml:"status"`
	Body   testOutputBody `yaml:"body"`
}

type testOutputBody struct {
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

type testRequest struct {
	Input  testInput  `yaml:"input"`
	Output testOutput `yaml:"output"`
}

type testCase struct {
	Name     string        `yaml:"name"`
	Requests []testRequest `yaml:"requests"`
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

func assertStringField(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
	t.Helper()
	if len(expected) >= 2 && expected[0] == '/' && expected[len(expected)-1] == '/' {
		pattern := expected[1 : len(expected)-1]
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Errorf("invalid regex pattern %q: %v", pattern, err)
			return
		}
		if !re.MatchString(actual) {
			assert.Fail(t, fmt.Sprintf("regex mismatch: pattern %q did not match %q", pattern, actual), msgAndArgs...)
		}
	} else {
		assert.Equal(t, expected, actual, msgAndArgs...)
	}
}

func TestE2E(t *testing.T) {
	var files []string
	err := fs.WalkDir(testFiles, "tests", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(p, ".yml") {
			files = append(files, p)
		}
		return nil
	})
	require.NoError(t, err, "failed to walk test files")
	require.NotEmpty(t, files, "no test files found")

	for _, file := range files {
		data, err := fs.ReadFile(testFiles, file)
		require.NoError(t, err, "failed to read test file: %s", file)

		var tf testFile
		err = yaml.Unmarshal(data, &tf)
		require.NoError(t, err, "failed to parse test file: %s", file)

		testPath := strings.TrimSuffix(strings.TrimPrefix(file, "tests/"), ".yml")
		require.NotEmpty(t, tf.Tests, "test file %s contains no test cases", file)

		for i, tc := range tf.Tests {
			t.Run(fmt.Sprintf("%s/%d/%s", testPath, i, tc.Name), func(t *testing.T) {
				t.Parallel()
				for ri, req := range tc.Requests {
					func() {
						files := make([]apiFile, len(req.Input.Files))
						for fi, f := range req.Input.Files {
							content, err := f.resolveContent()
							require.NoError(t, err, "[request %d] failed to resolve content for file %q", ri, f.Name)
							files[fi] = apiFile{
								Name:    f.Name,
								Content: base64.StdEncoding.EncodeToString([]byte(content)),
							}
						}

						reqBody := apiRequest{
							Runtime: req.Input.Runtime,
							Files:   files,
						}

						bodyBytes, err := json.Marshal(reqBody)
						require.NoError(t, err, "[request %d] failed to marshal request body", ri)

						url := fmt.Sprintf("%s/v1/run", serverURL)
						resp, err := http.Post(url, "application/json", bytes.NewReader(bodyBytes))
						require.NoError(t, err, "[request %d] failed to send request to %s", ri, url)
						defer func() { _ = resp.Body.Close() }()

						require.Equal(t, req.Output.Status, resp.StatusCode, "[request %d] unexpected HTTP status code", ri)

						if req.Output.Body.Error != "" {
							var errResp apiErrorResponse
							err = json.NewDecoder(resp.Body).Decode(&errResp)
							require.NoError(t, err, "[request %d] failed to decode error response body", ri)

							assert.Equal(t, req.Output.Body.Error, errResp.Error, "[request %d] error message mismatch", ri)
						} else {
							var apiResp apiResponse
							err = json.NewDecoder(resp.Body).Decode(&apiResp)
							require.NoError(t, err, "[request %d] failed to decode response body", ri)

							if req.Output.Body.Compile == nil {
								assert.Nil(t, apiResp.Compile, "[request %d] compile should be null", ri)
							} else {
								require.NotNil(t, apiResp.Compile, "[request %d] compile should not be null", ri)
								assertStringField(t, req.Output.Body.Compile.Stdout, decodeBase64(t, apiResp.Compile.Stdout, "compile stdout"), "[request %d] compile stdout mismatch", ri)
								assertStringField(t, req.Output.Body.Compile.Stderr, decodeBase64(t, apiResp.Compile.Stderr, "compile stderr"), "[request %d] compile stderr mismatch", ri)
								assertStringField(t, req.Output.Body.Compile.Output, decodeBase64(t, apiResp.Compile.Output, "compile output"), "[request %d] compile output mismatch", ri)
								assert.Equal(t, req.Output.Body.Compile.ExitCode, apiResp.Compile.ExitCode, "[request %d] compile exit_code mismatch", ri)
								assert.Equal(t, req.Output.Body.Compile.Status, apiResp.Compile.Status, "[request %d] compile status mismatch", ri)
								assert.Equal(t, req.Output.Body.Compile.Signal, apiResp.Compile.Signal, "[request %d] compile signal mismatch", ri)
							}

							if req.Output.Body.Run == nil {
								assert.Nil(t, apiResp.Run, "[request %d] run should be null", ri)
							} else {
								require.NotNil(t, apiResp.Run, "[request %d] run should not be null", ri)
								assertStringField(t, req.Output.Body.Run.Stdout, decodeBase64(t, apiResp.Run.Stdout, "run stdout"), "[request %d] run stdout mismatch", ri)
								assertStringField(t, req.Output.Body.Run.Stderr, decodeBase64(t, apiResp.Run.Stderr, "run stderr"), "[request %d] run stderr mismatch", ri)
								assertStringField(t, req.Output.Body.Run.Output, decodeBase64(t, apiResp.Run.Output, "run output"), "[request %d] run output mismatch", ri)
								assert.Equal(t, req.Output.Body.Run.ExitCode, apiResp.Run.ExitCode, "[request %d] run exit_code mismatch", ri)
								assert.Equal(t, req.Output.Body.Run.Status, apiResp.Run.Status, "[request %d] run status mismatch", ri)
								assert.Equal(t, req.Output.Body.Run.Signal, apiResp.Run.Signal, "[request %d] run signal mismatch", ri)
							}
						}
					}()
				}
			})
		}
	}
}
