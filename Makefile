
CMD = jpgo

SRC_PKGS=./ ./cmd/...

.PHONY: help generate build test check htmlc fuzz bench pprof-cpu install-dev-cmds

help:
	@echo "Please use \`make <target>' where <target> is one of"
	@echo "  test                    to run all the tests"
	@echo "  build                   to build the library and jp executable"
	@echo "  generate                to run codegen"


generate:
	go generate ${SRC_PKGS}

build:
	rm -f $(CMD)
	go build ${SRC_PKGS}
	rm -f cmd/$(CMD)/$(CMD) && cd cmd/$(CMD)/ && go build ./...
	mv cmd/$(CMD)/$(CMD) .

test: build
	go test -v ${SRC_PKGS}

check:
	go vet ${SRC_PKGS}
	golint ${SRC_PKGS}
	golangci-lint run

htmlc:
	go test -coverprofile="/tmp/jpcov"  && go tool cover -html="/tmp/jpcov" && unlink /tmp/jpcov

FUZZ_TARGET ?= FuzzParser
FUZZ_TIME ?= 30s

fuzz:
	go test -run=^$$ -fuzz=$(FUZZ_TARGET) -fuzztime=$(FUZZ_TIME) ./

bench:
	go test -bench . -cpuprofile cpu.out

pprof-cpu:
	go tool pprof ./go-jmespath.test ./cpu.out

install-dev-cmds:
	go install golang.org/x/lint/golint@latest
	go install golang.org/x/tools/cmd/stringer@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0
