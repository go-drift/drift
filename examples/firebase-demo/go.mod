module github.com/go-drift/drift/examples/firebase-demo

go 1.24.0

require (
	github.com/go-drift/drift v0.0.0
	github.com/go-drift/drift/plugins/firebase v0.0.0
)

require (
	golang.org/x/image v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace (
	github.com/go-drift/drift => ../..
	github.com/go-drift/drift/plugins/firebase => ../../plugins/firebase
)
