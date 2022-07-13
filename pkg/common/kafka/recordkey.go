package kafkacommon

const (
	// CreateCommit kafka record key const for creating a commit
	CreateCommit string = "create_commit"
	// CreateImageUpdate kafka record key const for creating an update
	CreateImageUpdate string = "create_image_update"
	// CreateInstaller kafka record key const for creating an installer
	CreateInstaller string = "create_installer"
	// CreateKickstart kafka record key const for creating and injecting a kickstart file
	CreateKickstart string = "create_kickstart"
)
