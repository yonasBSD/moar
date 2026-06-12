package main

import (
	"strconv"
	"strings"

	"github.com/walles/moor/v2/internal/linemetadata"
)

// Parses arguments starting with '+':
//
// - +1234 sets the initial line number to 1234 (one-based)
func parsePlusArgs(args []string) (*linemetadata.Index, []string) {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "+") {
			continue
		}

		targetIndex := parseLineNumber(arg[1:])
		if targetIndex == nil {
			// Let's pretend this is a file name
			continue
		}

		// Remove the target line number from the args
		//
		// Ref: https://stackoverflow.com/a/57213476/473672
		remainingArgs := make([]string, 0)
		remainingArgs = append(remainingArgs, args[:i]...)
		remainingArgs = append(remainingArgs, args[i+1:]...)

		return targetIndex, remainingArgs
	}

	return nil, args
}

func parseLineNumber(withoutPlus string) *linemetadata.Index {
	lineNumber, err := strconv.ParseInt(withoutPlus, 10, 32)
	if err != nil {
		return nil
	}
	if lineNumber < 0 {
		return nil
	}

	if lineNumber == 0 {
		// Special case +0 because that's what less does:
		// https://github.com/walles/moor/issues/316
		lineNumber = 1
	}

	targetIndex := linemetadata.IndexFromOneBased(int(lineNumber))
	return &targetIndex
}
