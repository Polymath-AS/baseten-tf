#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/test.sh [ci|unit|lint|vuln|build|docs|release-check|acceptance]

Suites:
  ci             Run unit tests, lint, vulnerability, build, docs, and release checks.
  unit           Run Go tests that do not require real Baseten credentials.
  lint           Run golangci-lint.
  vuln           Run govulncheck.
  build          Build the provider binary.
  docs           Regenerate provider docs and fail if docs/ changes.
  release-check  Validate GoReleaser configuration.
  acceptance     Run TestAcc tests. Requires TF_ACC=1 and BASETEN_API_KEY.
USAGE
}

suite="${1:-ci}"

run_unit() {
  go test ./...
}

run_lint() {
  golangci-lint run
}

run_vuln() {
  govulncheck ./...
}

run_build() {
  go build ./...
}

run_docs() {
  tfplugindocs generate --provider-name baseten
  git diff --exit-code -- docs
}

run_acceptance() {
  if [[ "${TF_ACC:-}" != "1" ]]; then
    echo "TF_ACC=1 is required for acceptance tests" >&2
    exit 1
  fi

  if [[ -z "${BASETEN_API_KEY:-}" ]]; then
    echo "BASETEN_API_KEY is required for acceptance tests" >&2
    exit 1
  fi

  go test ./... -run '^TestAcc' -count=1
}

run_release_check() {
  goreleaser check
}

case "$suite" in
  ci)
    run_unit
    run_lint
    run_vuln
    run_build
    run_docs
    run_release_check
    ;;
  unit)
    run_unit
    ;;
  lint)
    run_lint
    ;;
  vuln)
    run_vuln
    ;;
  build)
    run_build
    ;;
  docs)
    run_docs
    ;;
  release-check)
    run_release_check
    ;;
  acceptance)
    run_acceptance
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
