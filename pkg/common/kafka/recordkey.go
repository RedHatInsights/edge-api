// FIXME: golangci-lint
// nolint:revive
package kafkacommon

const (
	// RecordKeyCreateCommit kafka record key const for creating a commit
	RecordKeyCreateCommit string = "create_commit"
	// RecordKeyCreateImage kafka record key const for creating an image (wrapper for commit and installer)
	RecordKeyCreateImage string = "create_image"
	// RecordKeyCreateImageUpdate kafka record key const for creating an update
	RecordKeyCreateImageUpdate string = "create_image_update"
	// RecordKeyCreateInstaller kafka record key const for creating an installer
	RecordKeyCreateInstaller string = "create_installer"
	// RecordKeyCreateKickstart kafka record key const for creating and injecting a kickstart file
	RecordKeyCreateKickstart string = "create_kickstart"
)
