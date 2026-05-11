package platform

// PhotosService provides photo library access.
type PhotosService struct {
	// Permission for photo library access.
	Permission Permission
}

// Photos is the singleton photos service.
var Photos = &PhotosService{
	Permission: newPerm("photos"),
}
