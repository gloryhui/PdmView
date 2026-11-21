GO ?= go
VERSION ?= 0.1.0
export CGO_ENABLED := 0
go_files = find . -type d \( -name .git -o -name .local -o -name .out \) -prune -o -type f -name '*.go'

.PHONY: env fmt format-check test vet check probe package-macos

env:
	sh scripts/check-environment.sh

fmt:
	@toolchain_root="$$($(GO) env GOROOT)" && \
		$(go_files) -exec "$$toolchain_root/bin/gofmt" -w {} +

format-check:
	@toolchain_root="$$($(GO) env GOROOT)" && \
		unformatted="$$($(go_files) -exec "$$toolchain_root/bin/gofmt" -l {} +)" && \
		if [ -n "$$unformatted" ]; then printf '%s\n' "$$unformatted"; exit 1; fi

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: format-check test vet

probe:
	$(GO) run ./cmd/mygo-probe

package-macos: export GO := $(GO)
package-macos: export VERSION := $(VERSION)
package-macos:
	sh scripts/package-macos.sh
