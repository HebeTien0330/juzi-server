#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

PROTOC_GEN_GO="${HOME}/go/bin/protoc-gen-go"
if [ ! -x "${PROTOC_GEN_GO}" ]; then
	echo "protoc-gen-go not found at ${PROTOC_GEN_GO}"
	echo "Run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
	exit 1
fi

PROTO_ROOT="../pb"
OUT_ROOT="../../"

compile() {
	local layer="$1"
	local dir="${PROTO_ROOT}/${layer}"
	if ! compgen -G "${dir}/*.proto" > /dev/null; then
		return 0
	fi
	echo "Compiling ${layer}"
	protoc \
		--plugin=protoc-gen-go="${PROTOC_GEN_GO}" \
		--go_out="${OUT_ROOT}" \
		--proto_path="${PROTO_ROOT}" \
		"${dir}"/*.proto
}

compile common
compile public
compile internal
echo "Done."
