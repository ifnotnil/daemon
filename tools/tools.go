//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/itchyny/gojq/cmd/gojq"
	_ "github.com/vektra/mockery/v3"
	_ "mvdan.cc/sh/v3/cmd/shfmt"
)

func main() {
}
