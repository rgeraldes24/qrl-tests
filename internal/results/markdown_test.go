package results

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2/types"
	"github.com/stretchr/testify/require"
)

func TestSummarizeWritesMarkdown(t *testing.T) {
	root := t.TempDir()
	summary, err := Summarize(root, []Outcome{{
		Name: "execution",
		reports: []types.Report{{
			SuiteDescription: "ABI E2E suite",
			SuiteSucceeded:   true,
			SpecReports: []types.SpecReport{{
				LeafNodeType: types.NodeTypeIt,
				State:        types.SpecStatePassed,
			}},
		}},
	}})
	require.NoError(t, err)

	markdown, err := os.ReadFile(filepath.Join(root, MarkdownFileName))
	require.NoError(t, err)
	require.Equal(t, summary.markdown(), string(markdown))
}

func TestMarkdownPassingSuites(t *testing.T) {
	summary := Summary{
		Result: "passed",
		Lanes: []LaneSummary{{
			Name:  "execution",
			Class: ClassPassed,
			suites: []suiteSummary{
				{Name: "ABI E2E suite", Class: ClassPassed, Counts: Counts{Specs: 2, Passed: 2}},
				{Name: "API E2E suite", Class: ClassPassed, Counts: Counts{Specs: 3, Passed: 3}},
			},
		}},
	}

	want := strings.Join([]string{
		"## E2E: passed",
		"",
		"### execution",
		"",
		"| Suite | Result |",
		"|---------|--------:|",
		"| ABI E2E suite | 2/2 |",
		"| API E2E suite | 3/3 |",
		"",
	}, "\n")
	require.Equal(t, want, summary.markdown())
}

func TestMarkdownFailedSuiteDetails(t *testing.T) {
	summary := Summary{
		Result: "failed",
		Lanes: []LaneSummary{{
			Name:  "execution",
			Class: ClassAssertion,
			Error: "exit status 1",
			suites: []suiteSummary{
				{Name: "API E2E suite", Class: ClassPassed, Counts: Counts{Specs: 28, Passed: 28}},
				{
					Name:  "ABI E2E suite",
					Class: ClassAssertion,
					Counts: Counts{
						Specs:  2,
						Passed: 1,
						Failed: 1,
					},
					Failures: []Failure{{
						Spec:     "ABI decodes events",
						State:    "failed",
						Message:  "expected 1,\ngot 2",
						Location: "calls_test.go:12",
					}},
					SuiteFailures:   []string{"suite cleanup failed"},
					UnexpectedSkips: []string{"ABI encodes arrays"},
				},
			},
		}},
	}

	want := strings.Join([]string{
		"## E2E: failed",
		"",
		"### execution",
		"",
		"| Suite | Result |",
		"|---------|--------:|",
		"| API E2E suite | 28/28 |",
		"| ABI E2E suite | 1/2 failed |",
		"",
		"#### ABI E2E suite failures",
		"",
		"- **failed** `ABI decodes events` (calls_test.go:12)",
		"  expected 1,",
		"  got 2",
		"- **suite** suite cleanup failed",
		"- **skipped** `ABI encodes arrays`",
	}, "\n")
	got := summary.markdown()
	require.Equal(t, want, got)

	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		require.NoError(t, err)
		defer file.Close()

		_, err = file.WriteString(got)
		require.NoError(t, err)
	}
}

func TestMarkdownInfrastructureFailureWithoutSuites(t *testing.T) {
	summary := Summary{
		Result: "failed",
		Lanes: []LaneSummary{{
			Name:  "consensus",
			Class: ClassInfrastructure,
			Error: "network did not become ready\nconsensus REST unavailable",
		}},
	}

	want := strings.Join([]string{
		"## E2E: failed",
		"",
		"### consensus",
		"",
		"**Result:** error",
		"",
		"### consensus details",
		"",
		"```",
		"network did not become ready",
		"consensus REST unavailable",
		"```",
	}, "\n")
	require.Equal(t, want, summary.markdown())
}
