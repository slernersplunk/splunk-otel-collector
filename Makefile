### VARIABLES

ADDLICENSE= addlicense

# All source code and documents. Used in spell check.
ALL_DOC := $(shell find . \( -name "*.md" -o -name "*.yaml" \) \
                                -type f | sort)

# ALL_MODULES includes ./* dirs (excludes . dir)
ALL_MODULES := $(shell find . -type f -name "go.mod" -exec dirname {} \; | sort | egrep  '^./' )

# All source code excluding any third party code and excluding the testbed.
# This is the code that we want to run tests for and lint, staticcheck, etc.
ALL_SRC := $(shell find . -name '*.go' \
							-not -path './examples/*' \
							-not -path './tests/*' \
							-type f | sort)

# ALL_PKGS is the list of all packages where ALL_SRC files reside.
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

ALL_TESTS_DIRS := $(shell find tests -name *_test.go | xargs dirname | uniq | sort -r)

# BUILD_TYPE should be one of (dev, release).
BUILD_TYPE?=release

GIT_SHA=$(shell git rev-parse --short HEAD)
GO_ACC=go-acc
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS)
GOSEC=gosec
GOTEST=go test
GOTEST_OPT?= -v -race -timeout 180s
IMPI=impi
LINT=golangci-lint
MISSPELL=misspell -error
MISSPELL_CORRECTION=misspell -w
STATIC_CHECK=staticcheck

BUILD_INFO_IMPORT_PATH=github.com/signalfx/splunk-otel-collector/internal/version
BUILD_X1=-X $(BUILD_INFO_IMPORT_PATH).GitHash=$(GIT_SHA)
ifdef VERSION
BUILD_X2=-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)
endif
BUILD_X3=-X $(BUILD_INFO_IMPORT_PATH).BuildType=$(BUILD_TYPE)
BUILD_INFO=-ldflags "${BUILD_X1} ${BUILD_X2} ${BUILD_X3}"

SMART_AGENT_RELEASE=v5.9.1

### FUNCTIONS

# Function to execute a command. Note the empty line before endef to make sure each command
# gets executed separately instead of concatenated with previous one.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

### TARGETS

all-srcs:
	@echo $(ALL_SRC) | tr ' ' '\n' | sort

all-pkgs:
	@echo $(ALL_PKGS) | tr ' ' '\n' | sort

.DEFAULT_GOAL := all

.PHONY: all
all: checklicense impi lint misspell test otelcol

.PHONY: test
test:
	$(GOTEST) $(GOTEST_OPT) $(ALL_PKGS)

.PHONY: integration-test
integration-test:
	@set -e; for dir in $(ALL_TESTS_DIRS); do \
	  echo "go test ./... in $${dir}"; \
	  (cd "$${dir}" && \
	   $(GOTEST) -v -timeout 5m -count 1 ./... ); \
	done


.PHONY: test-with-cover
test-with-cover:
	@echo Verifying that all packages have test files to count in coverage
	@echo pre-compiling tests
	@time go test -i $(ALL_PKGS)
	$(GO_ACC) $(ALL_PKGS)
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: addlicense
addlicense:
	$(ADDLICENSE) -c 'Splunk, Inc.' $(ALL_SRC)

.PHONY: checklicense
checklicense:
	@ADDLICENSEOUT=`$(ADDLICENSE) -check $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENSEOUT" ]; then \
			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENSEOUT\n"; \
			echo "Use 'make addlicense' to fix this."; \
			exit 1; \
		else \
			echo "Check License finished successfully"; \
		fi

DEPENDABOT_PATH=./.github/dependabot.yml
.PHONY: gendependabot
gendependabot:
	@echo "Recreate dependabot.yml file"
	@echo "# File generated by \"make gendependabot\"; DO NOT EDIT.\n" > ${DEPENDABOT_PATH}
	@echo "version: 2" >> ${DEPENDABOT_PATH}
	@echo "updates:" >> ${DEPENDABOT_PATH}
	@echo "Add entry for \"/\""
	@echo "  - package-ecosystem: \"gomod\"\n    directory: \"/\"\n    schedule:\n      interval: \"weekly\"" >> ${DEPENDABOT_PATH}
	@set -e; for dir in $(ALL_MODULES); do \
		(echo "Add entry for \"$${dir:1}\"" && \
		  echo "  - package-ecosystem: \"gomod\"\n    directory: \"$${dir:1}\"\n    schedule:\n      interval: \"weekly\"" >> ${DEPENDABOT_PATH} ); \
	done

.PHONY: misspell
misspell:
	$(MISSPELL) $(ALL_DOC)

.PHONY: misspell-correction
misspell-correction:
	$(MISSPELL_CORRECTION) $(ALL_DOC)

.PHONY: lint-gosec
lint-gosec:
	# TODO: Consider to use gosec from golangci-lint
	$(GOSEC) -quiet -exclude=G104 $(ALL_PKGS)

.PHONY: lint-static-check
lint-static-check:
	@STATIC_CHECK_OUT=`$(STATIC_CHECK) $(ALL_PKGS) 2>&1`; \
		if [ "$$STATIC_CHECK_OUT" ]; then \
			echo "$(STATIC_CHECK) FAILED => static check errors:\n"; \
			echo "$$STATIC_CHECK_OUT\n"; \
			exit 1; \
		else \
			echo "Static check finished successfully"; \
		fi

.PHONY: lint
lint: lint-static-check
	$(LINT) run

.PHONY: impi
impi:
	@$(IMPI) --local github.com/signalfx/splunk-otel-collector --scheme stdThirdPartyLocal ./...

.PHONY: install-tools
install-tools:
	go install github.com/client9/misspell/cmd/misspell
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/google/addlicense
	go install github.com/jstemmer/go-junit-report
	go install github.com/ory/go-acc
	go install github.com/pavius/impi/cmd/impi
	go install github.com/securego/gosec/cmd/gosec
	go install honnef.co/go/tools/cmd/staticcheck
	go install github.com/tcnksm/ghr

.PHONY: otelcol
otelcol:
	go generate ./...
	GO111MODULE=on CGO_ENABLED=0 go build -o ./bin/otelcol_$(GOOS)_$(GOARCH)$(EXTENSION) $(BUILD_INFO) ./cmd/otelcol
	ln -sf otelcol_$(GOOS)_$(GOARCH)$(EXTENSION) ./bin/otelcol

.PHONY: add-tag
add-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Adding tag ${TAG}"
	@git tag -a ${TAG} -s -m "Version ${TAG}"

.PHONY: delete-tag
delete-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Deleting tag ${TAG}"
	@git tag -d ${TAG}

.PHONY: docker-otelcol
docker-otelcol:
	GOOS=linux $(MAKE) otelcol
	cp ./bin/otelcol_linux_amd64 ./cmd/otelcol/otelcol
	docker build -t otelcol ./cmd/otelcol/
	rm ./cmd/otelcol/otelcol

.PHONY: binaries-all-sys
binaries-all-sys: binaries-darwin_amd64 binaries-linux_amd64 binaries-linux_arm64 binaries-windows_amd64

.PHONY: binaries-darwin_amd64
binaries-darwin_amd64:
	GOOS=darwin  GOARCH=amd64 $(MAKE) otelcol

.PHONY: binaries-linux_amd64
binaries-linux_amd64:
	GOOS=linux   GOARCH=amd64 $(MAKE) otelcol

.PHONY: binaries-linux_arm64
binaries-linux_arm64:
	GOOS=linux   GOARCH=arm64 $(MAKE) otelcol

.PHONY: binaries-windows_amd64
binaries-windows_amd64:
	GOOS=windows GOARCH=amd64 EXTENSION=.exe $(MAKE) otelcol

.PHONY: deb-rpm-package
%-package: ARCH ?= amd64
%-package:
	$(MAKE) binaries-linux_$(ARCH)
	docker build -t otelcol-fpm internal/buildscripts/packaging/fpm
	docker run --rm -v $(CURDIR):/repo -e PACKAGE=$* -e VERSION=$(VERSION) -e ARCH=$(ARCH) -e SMART_AGENT_RELEASE=$(SMART_AGENT_RELEASE) otelcol-fpm
