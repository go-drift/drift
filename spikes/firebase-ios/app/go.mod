module example.com/firebasespike/app

go 1.24.0

require (
	example.com/firebasespike v0.0.0-00010101000000-000000000000
	github.com/go-drift/drift v0.0.0
)

require (
	golang.org/x/image v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	example.com/firebasespike => ../firebasespike
	github.com/go-drift/drift => ../../..
)
