#!/bin/bash

set -euo pipefail

run_name=$1
artifacts=${2:-""}

if [ "$artifacts" == "master" ]; then
    artifacts="gce-scale-cluster-master"
fi

output="$HOME/debug/${run_name}"
mkdir -p "${output}"

prefix="gs://kubernetes-jenkins/logs/${run_name}"

if [ -n "$artifacts" ]; then
    gsutil -m  cp -R "${prefix}/artifacts/${artifacts}" "${output}"
else
    gsutil -m  cp "${prefix}/build-log.txt" "${output}"
fi
echo "Downloaded to ${output}"
