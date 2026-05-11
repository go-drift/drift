package platform

// ContactsService provides contacts/address book access.
type ContactsService struct {
	// Permission for contacts access.
	Permission Permission
}

// Contacts is the singleton contacts service.
var Contacts = &ContactsService{
	Permission: newPerm("contacts"),
}
