#!/bin/sh
# This script is intended to run as a pre-commit or pre-push hook for a Git
# repository. It verifies that the code is in a useful state before pushing.
git go presubmit
