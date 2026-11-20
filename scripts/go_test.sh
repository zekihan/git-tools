#!/bin/sh

set -e

SCRIPT_PATH=$0
if [ ! -f "$SCRIPT_PATH" ]; then
    SCRIPT_PATH=$(command -v -- "$SCRIPT_PATH")
fi

DIR=$( CDPATH='' cd -P -- "$(dirname -- "$SCRIPT_PATH")" && pwd -P )

cd "${DIR}/.."

go test "$@"
