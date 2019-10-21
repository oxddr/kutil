#!/bin/bash

set -euo pipefail

run_name=$1
artifacts=${2:-""}

if [ "$artifacts" == "master" ]; then
    artifacts="gce-scale-cluster-master"
fi

case $run_name in
    *gke*)
        bucket=gs://gke-scalability-prow
        ;;
    *gce*)
        bucket=gs://kubernetes-jenkins
        ;;
esac


output="$HOME/debug/${run_name}"

mkdir -p "${output}"
if [ -n "$artifacts" ]; then
    gsutil -m  cp -R "${bucket}"/logs/"${run_name}"/artifacts/"${artifacts}" "${output}"
else
    gsutil -m  cp "${bucket}"/logs/"${run_name}"/build-log.txt "${output}"
fi
echo "Downloaded to ${output}"
