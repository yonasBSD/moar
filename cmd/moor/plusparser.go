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

		lineNumber, err := strconv.ParseInt(arg[1:], 10, 32)
		if err != nil {
			// Let's pretend this is a file name
			continue
		}
		if lineNumber < 0 {
			// Pretend this is a file name
			continue
		}

		// Remove the target line number from the args
		//
		// Ref: https://stackoverflow.com/a/57213476/473672
		remainingArgs := make([]string, 0)
		remainingArgs = append(remainingArgs, args[:i]...)
		remainingArgs = append(remainingArgs, args[i+1:]...)

		if lineNumber == 0 {
			// Ignore +0 because that's what less does:
			// https://github.com/walles/moor/issues/316
			return nil, remainingArgs
		}

		returnMe := linemetadata.IndexFromOneBased(int(lineNumber))
		return &returnMe, remainingArgs
	}

	return nil, args
}
