export GOPATH=`pwd`
mkdir -p vnetbin

os=$1
arch=$2
prg=$3
if [ x$os = x"" ]; then
    os="linux"
fi
if [ x$arch = x"" ]; then
    arch="amd64"
fi
version="v2.0.0"

GOOS=$os GOARCH=$arch go build -o vnetbin/govnet_${os}${arch}_`date '+%Y%m%d'` -ldflags \
"-extldflags '-static' -X main.goVersion=`go version|awk '{printf $3}'` -X main.buildTime=`date +%Y%m%d/%H:%M:%S` -X main.commitId=`git rev-parse HEAD` -X main.version=$version" \
src/main/govnet.go

