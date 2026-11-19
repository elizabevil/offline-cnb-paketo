#!/usr/bin/env bash
set -euo pipefail

cd "${{ci.workspace}}/${{variables.SOURCE_CODE_DIR}}"
repo_dir="$(pwd -P)"
version="${{variables.APP_VERSION}}"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
    printf 'APP_VERSION 格式不正确：%s\n' "$version" >&2
    exit 2
fi

output_dir="$repo_dir/build/smart"
cache_dir="$repo_dir/build/.cache/smart"
mkdir -p "$output_dir" "$cache_dir/go-mod" "$cache_dir/go-build"

docker run --rm -i \
    --user "$(id -u):$(id -g)" \
    --workdir /workspace \
    --mount "type=bind,source=$repo_dir,target=/workspace,readonly" \
    --mount "type=bind,source=$output_dir,target=/out" \
    --mount "type=bind,source=$cache_dir/go-mod,target=/go/pkg/mod" \
    --mount "type=bind,source=$cache_dir/go-build,target=/tmp/go-build" \
    --env "APP_VERSION=$version" \
    --env "GOPROXY=${GOPROXY:-https://mirrors.tencent.com/go}" \
    --env GOMODCACHE=/go/pkg/mod \
    --env GOCACHE=/tmp/go-build \
    --env GOTOOLCHAIN=local \
    --env CGO_ENABLED=0 \
    --env GOOS=linux \
    --env GOARCH=amd64 \
    mirror.ccs.tencentyun.com/library/golang:1.26.4-bookworm \
    bash -s <<'BUILD'
set -euo pipefail
umask 022

go version

work="$(mktemp -d /out/.slug-build.XXXXXX)"
trap 'rm -rf -- "$work"' EXIT

runtime="$work/runtime"
process="$work/process"
package="$work/package"
mkdir -p "$runtime/app/bin" "$runtime/app/extra" "$process/app" "$package"

go build -mod=readonly -trimpath -buildvcs=false \
    -ldflags "-s -w -X code.cwoa.net/rd-fy18-canway-cmdb/cc-enhancement/internal/version.Version=$APP_VERSION" \
    -o "$runtime/app/cc-enhancement" ./cmd/server

install -m 0755 script/smart/extra-data/bin/start-enhancement.sh "$runtime/app/bin/"
install -m 0755 script/extra/envsubst-linux-amd64 "$runtime/app/extra/envsubst"
install -m 0644 script/smart/extra-data/configure/cc-enhancement.yaml.tmpl "$runtime/app/extra/"

sed -E "s/^app_version:.*$/app_version: \"$APP_VERSION\"/" \
    script/smart/app_desc.yaml > "$package/app_desc.yaml"
cp "$package/app_desc.yaml" "$package/app_desc.yml"
cp script/smart/logo.png "$package/"

# app_desc.yaml 中模块是 web，source_dir 是 .，因此两层放在包根目录。
printf '%s\n' slug-layers > "$package/.Version"
printf '%s\n' 'web: /app/bin/start-enhancement.sh' > "$process/app/Procfile"

tar --owner=0 --group=0 --numeric-owner \
    -C "$runtime" -czf "$package/layer.tar.gz" \
    app/cc-enhancement app/bin app/extra
tar --owner=0 --group=0 --numeric-owner \
    -C "$process" -czf "$package/web.Procfile.tar.gz" app/Procfile

archive="cmdb_enhancement-$APP_VERSION-linux-amd64_paas3.tar.gz"
dest="/out/$APP_VERSION-linux-amd64"

tar -C "$package" -czf "$work/$archive" .
(cd "$work" && sha256sum "$archive" > "$archive.sha256")

mkdir -p "$dest"
mv "$work/$archive" "$work/$archive.sha256" "$dest/"
(cd "$dest" && sha256sum -c "$archive.sha256")
BUILD

printf 'S-Mart 包：%s/%s-linux-amd64/cmdb_enhancement-%s-linux-amd64_paas3.tar.gz\n' \
    "$output_dir" "$version" "$version"