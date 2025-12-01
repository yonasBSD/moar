package search

import (
	"regexp"
	"unicode"
)

type Search struct {
	String  string
	pattern *regexp.Regexp
}

func For(s string) Search {
	search := Search{}
	search.For(s)
	return search
}

func (search *Search) For(s string) {
	search.String = s
	search.pattern = toPattern(s)
}

func (search *Search) Stop() {
	search.String = ""
	search.pattern = nil
}

func (search Search) Inactive() bool {
	return search.pattern == nil
}

func (search Search) Matches(line string) bool {
	if search.pattern == nil {
		return false
	}
	return search.pattern.MatchString(line)
}

// getMatchRanges locates one or more regexp matches in a string
func (search Search) GetMatchRanges(String string) *MatchRanges {
	if search.Inactive() {
		return nil
	}

	return &MatchRanges{
		Matches: toRunePositions(search.pattern.FindAllStringIndex(String, -1), String),
	}
}

// Convert byte indices to rune indices
func toRunePositions(byteIndices [][]int, matchedString string) [][2]int {
	var returnMe [][2]int
	if len(byteIndices) == 0 {
		// Nothing to see here, move along
		return returnMe
	}

	runeIndex := 0
	byteIndicesToRuneIndices := make(map[int]int, 0)
	for byteIndex := range matchedString {
		byteIndicesToRuneIndices[byteIndex] = runeIndex

		runeIndex++
	}

	// If a match touches the end of the string, that will be encoded as one
	// byte past the end of the string. Therefore we must add a mapping for
	// first-index-after-the-end.
	byteIndicesToRuneIndices[len(matchedString)] = runeIndex

	for _, bytePair := range byteIndices {
		fromRuneIndex := byteIndicesToRuneIndices[bytePair[0]]
		toRuneIndex := byteIndicesToRuneIndices[bytePair[1]]
		returnMe = append(returnMe, [2]int{fromRuneIndex, toRuneIndex})
	}

	return returnMe
}

// toPattern compiles a search string into a pattern.
//
// If the string contains only lower-case letter the pattern will be case insensitive.
//
// If the string is empty the pattern will be nil.
//
// If the string does not compile into a regexp the pattern will match the string verbatim
func toPattern(compileMe string) *regexp.Regexp {
	if len(compileMe) == 0 {
		return nil
	}

	hasUppercase := false
	for _, char := range compileMe {
		if unicode.IsUpper(char) {
			hasUppercase = true
		}
	}

	// Smart case; be case insensitive unless there are upper case chars
	// in the search string
	prefix := "(?i)"
	if hasUppercase {
		prefix = ""
	}

	pattern, err := regexp.Compile(prefix + compileMe)
	if err == nil {
		// Search string is a regexp
		return pattern
	}

	pattern, err = regexp.Compile(prefix + regexp.QuoteMeta(compileMe))
	if err == nil {
		// Pattern matching the string exactly
		return pattern
	}

	// Unable to create a match-string-verbatim pattern
	panic(err)
}
