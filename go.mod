module github.com/ifnotnil/daemon

go 1.25

// Test dependencies. They will not be pushed downstream as indirect ones.
require (
	github.com/stretchr/testify v1.12.1
	go.uber.org/goleak v1.3.0
)

require (
	github.com/stretchr/objx v0.5.3 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)
