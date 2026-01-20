package core

// MountRoot inflates and mounts the root widget with the provided build owner.
func MountRoot(root Widget, owner *BuildOwner) Element {
	element := inflateWidget(root, owner)
	element.Mount(nil, nil)
	return element
}
