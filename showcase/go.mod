module github.com/go-drift/drift/showcase

go 1.24.0

require (
	github.com/go-drift/drift v0.0.0
	github.com/go-drift/drift/plugins/imagepicker v0.0.0
	github.com/go-drift/drift/plugins/location v0.0.0
	github.com/go-drift/drift/plugins/notifications v0.0.0
	github.com/go-drift/drift/plugins/permissions v0.0.0
	github.com/go-drift/drift/plugins/securestorage v0.0.0
)

require (
	golang.org/x/image v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/go-drift/drift => ../
	github.com/go-drift/drift/plugins/imagepicker => ../plugins/imagepicker
	github.com/go-drift/drift/plugins/location => ../plugins/location
	github.com/go-drift/drift/plugins/notifications => ../plugins/notifications
	github.com/go-drift/drift/plugins/permissions => ../plugins/permissions
	github.com/go-drift/drift/plugins/securestorage => ../plugins/securestorage
)
