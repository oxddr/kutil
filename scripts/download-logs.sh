#!/bin/bash

set -euo pipefail

run_name=$1
dirs=${2:-"gce-scale-cluster-master"}

output="$HOME/debug/${run_name}"

mkdir -p "${output}"
gsutil -m  cp -R gs://kubernetes-jenkins/logs/"${run_name}"/artifacts/"${dirs}" "${output}"
