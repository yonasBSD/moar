Use `test.sh` to run all tests. In addition to just running all tests, that
script will do linting, some cross compiling and more.

The `twin` directory contains the text windowing library used by `moor`. It is
used for screen output. See its README for details.

Inside of `twin`, log levels are:
- Debug: Only to be show when the user explicitly enables debug logging
- Info: Goes into crash reports, can be helpful as crash context
- Error: Triggers crash reports, these things should be fixed
