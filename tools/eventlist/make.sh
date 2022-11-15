#!/bin/bash

# -------------------------------------------------------
# Copyright (c) 2022 Arm Limited. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
# -------------------------------------------------------

# usage
usage() {
  echo ""
  echo "Usage:"
  echo "  make.sh <command> [<args>]"
  echo ""
  echo "commands:"
  echo "  build           : Build executable"
  echo "  clean           : Remove build artifacts"
  echo "  coverage        : Run tests with coverage info"
  echo "  coverage-report : Generate html coverage report"
  echo "  format          : Align indentation and format code"
  echo "  lint            : Run linter"
  echo "  test            : Run all tests"
  echo ""
  echo "args:"
  echo "  -arch           : Target architecture for e.g amd64 etc"
  echo "  -os             : Target operating system for e.g windows, linux, darwin etc"
}

if [ $# -eq 0 ]
then
  usage
  exit 0
fi

for cmdline in "$@"
do
  if [[ "${cmdline}" == "help" || "${cmdline}" == "-h" || "${cmdline}" == "--help" ]]; then
    usage
    exit 0
  fi
  arg="${cmdline}"
  args+=("${arg}")
done

go run cmd/make/make.go "${args[@]}"

RESULT=$?
if [ $RESULT -ne 0 ]; then
  usage
  exit 1
fi
exit 0