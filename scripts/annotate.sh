#!/bin/bash
#
# Annotate K8s object with ~13k of data

set -eu

TXT=$(tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w 25000 | head -n 1)
kubectl annotate --overwrite "$@" label="$TXT"
