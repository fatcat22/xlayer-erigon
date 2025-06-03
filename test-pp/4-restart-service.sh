#!/bin/bash
set -e
# set -x

make stop-old
sleep 5

make run-new