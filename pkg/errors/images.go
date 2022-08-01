package errors

// ImageNameUndefinedError indicates the image name is not defined
type ImageNameUndefinedError struct{}

func (e *ImageNameUndefinedError) Error() string {
	return "image name is not defined"
}

// OrgIDNotSetError indicates the account was nil
type OrgIDNotSetError struct{}

func (e *OrgIDNotSetError) Error() string {
	return "Org ID is not set"
}

// ImageNameAlreadyExistsError indicates the image with supplied name already exists
type ImageNameAlreadyExistsError struct{}

func (e *ImageNameAlreadyExistsError) Error() string {
	return "image with supplied name already exists"
}

// PackageNameDoesNotExistError indicates the image with supplied name already exists
type PackageNameDoesNotExistError struct{}

func (e *PackageNameDoesNotExistError) Error() string {
	return "package name does not exist"
}
