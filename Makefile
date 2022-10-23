OUTPUT_DIR=./bin
VERSION=`cat VERSION`
GIT_COMMIT=`git rev-list -1 HEAD | cut -c1-8`

all: fbauthtool

fbauthtool:
	@CGO_ENABLED=0 go build -o $(OUTPUT_DIR)/fbauthtool -ldflags "-X 'main.version=${VERSION}' -X 'main.gitCommit=${GIT_COMMIT}'" ./cmd/fbauthtool/main.go

clean:
	-@rm -r $(OUTPUT_DIR)/* 2> /dev/null || true
