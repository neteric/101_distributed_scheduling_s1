#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail




TARGET_SOURCE=(
  k8s-admission-webhook=cmd/webhook
)
GO_PACKAGE="github.com/VTeam/k8s-webhook-template"


function util:host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

function util::get_version() {
  git describe --tags --dirty
}

function util::version_ldflags() {
  # Git information
  GIT_VERSION=$(util::get_version)
  GIT_COMMIT_HASH=$(git rev-parse HEAD)
  if git_status=$(git status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
    GIT_TREESTATE="clean"
  else
    GIT_TREESTATE="dirty"
  fi
  BUILDDATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  LDFLAGS="-X github.com/VTeam/k8s-webhook-template/pkg/version.gitVersion=${GIT_VERSION} \
                        -X github.com/VTeam/k8s-webhook-template/pkg/version.gitCommit=${GIT_COMMIT_HASH} \
                        -X github.com/VTeam/k8s-webhook-template/pkg/version.gitTreeState=${GIT_TREESTATE} \
                        -X github.com/VTeam/k8s-webhook-template/pkg/version.buildDate=${BUILDDATE}"
  echo $LDFLAGS
}

function util::get_target_source() {
  local target=$1
  for s in "${TARGET_SOURCE[@]}"; do
    if [[ "$s" == ${target}=* ]]; then
      echo "${s##${target}=}"
      return
    fi
  done
}