# Validation

Use `MOOR_SKIP_INTERACTIVE_TESTS=1 ./test.sh` to run all tests. In addition to
just running all tests, that script will do linting, some cross compiling and
more.

# PR Best Practices

Always run `./test.sh` before making any PRs.

When fixing bugs, making one commit for the regression test and another one (or
more) for the fix makes the PR easier to review.

# Releases

Release messages go into annotated tags. Please look at the ten most recent
annotated tags for style guidance. The basis for all those messages are user
visible changes since last release.
