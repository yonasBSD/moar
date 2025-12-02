package search

import (
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestToPattern(t *testing.T) {
	assert.Assert(t, toPattern("") == nil)

	// Test regexp matching
	assert.Assert(t, toPattern("G.*S").MatchString("GRIIIS"))
	assert.Assert(t, !toPattern("G.*S").MatchString("gRIIIS"))

	// Test case insensitive regexp matching
	assert.Assert(t, toPattern("g.*s").MatchString("GRIIIS"))
	assert.Assert(t, toPattern("g.*s").MatchString("gRIIIS"))

	// Test non-regexp matching
	assert.Assert(t, toPattern(")G").MatchString(")G"))
	assert.Assert(t, !toPattern(")G").MatchString(")g"))

	// Test case insensitive non-regexp matching
	assert.Assert(t, toPattern(")g").MatchString(")G"))
	assert.Assert(t, toPattern(")g").MatchString(")g"))
}

func BenchmarkMatch(b *testing.B) {
	sourceBytes, err := os.ReadFile("../../sample-files/large-git-log-patch-no-color.txt")
	assert.NilError(b, err)

	fileContents := string(sourceBytes)
	b.SetBytes(int64(len(fileContents)))

	lines := strings.Split(fileContents, "\n")

	// Same as in benchmarkSearch() in search-linescanner_test.go
	search := For("This won't match anything")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, line := range lines {
			search.Matches(line)
		}
	}
}
