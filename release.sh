#!/bin/bash

#!/bin/bash

: ${TARGET_ARCHS:="amd64 arm64"}
: ${TARGET_OS:="linux darwin"}

function compile() {
  local name=$1 ;shift
  local arch=$1 ; shift
  local os=$1 ; shift
  local cmd_name=$1; shift
  local output="release/${arch}_${os}/$name"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags='-w -s -extldflags "-static"' -o "$output" "./cmd/$cmd_name"
}

for arch in ${TARGET_ARCHS}
do
  for os in ${TARGET_OS}
  do
    echo "Building ${arch} ${os}"
    mkdir -p "release/${arch}_${os}"
    compile mcp-imap "$arch" "$os" mcp

    (cd "release/${arch}_${os}"
    tar zcvf "../mcp-imap_${arch}_${os}.tgz" mcp-imap >/dev/null
    )
  done
done
