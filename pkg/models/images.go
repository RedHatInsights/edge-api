package models

import (
	"errors"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ImageSet represents a collection of images
type ImageSet struct {
	Model
	Name    string  `json:"Name"`
	Version int     `json:"Version" gorm:"default:1"`
	Account string  `json:"Account"`
	OrgID   string  `json:"org_id" gorm:"index"`
	Images  []Image `json:"Images"`
}

// ImageUpdateAvailable contains image and differences between current and available commits
type ImageUpdateAvailable struct {
	Image       Image       `json:"Image"`
	PackageDiff PackageDiff `json:"PackageDiff"`
}

// PackageDiff provides package difference details between current and available commits
type PackageDiff struct {
	Added    []InstalledPackage `json:"Added"`
	Removed  []InstalledPackage `json:"Removed"`
	Upgraded []InstalledPackage `json:"Upgraded"`
}

// ImageInfo contains Image with updates available and rollback image
type ImageInfo struct {
	Image            Image                   `json:"Image"`
	UpdatesAvailable *[]ImageUpdateAvailable `json:"UpdatesAvailable,omitempty"`
	Rollback         *Image                  `json:"RollbackImage,omitempty"`
}

const (
	// DistributionCantBeNilMessage is the error message when a distribution is nil
	DistributionCantBeNilMessage = "distribution can't be empty"
	// ArchitectureCantBeEmptyMessage is the error message when the architecture is empty
	ArchitectureCantBeEmptyMessage = "architecture can't be empty"
	// NameCantBeInvalidMessage is the error message when the name is invalid
	NameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// ImageTypeNotAccepted is the error message when an image type is not accepted
	ImageTypeNotAccepted = "this image type is not accepted"
	// ImageNameAlreadyExists is the error message when an image name alredy exists
	ImageNameAlreadyExists = "this image name is already in use"
	// NoOutputTypes is the error message when the output types list is empty
	NoOutputTypes = "an output type is required"

	// ImageTypeInstaller is the installer image type on Image Builder
	ImageTypeInstaller = "rhel-edge-installer"
	// ImageTypeCommit is the installer image type on Image Builder
	ImageTypeCommit = "rhel-edge-commit"

	// ImageStatusCreated is for when an image is created
	ImageStatusCreated = "CREATED"
	// ImageStatusBuilding is for when an image is building
	ImageStatusBuilding = "BUILDING"
	// ImageStatusError is for when an image is on a error state
	ImageStatusError = "ERROR"
	// ImageStatusSuccess is for when an image is available to the user
	ImageStatusSuccess = "SUCCESS"
	// ImageStatusInterrupted is for when an image build is interrupted
	ImageStatusInterrupted = "INTERRUPTED"
	// ImageStatusPending is for when an image or installer is waiting to be built
	ImageStatusPending = "PENDING"

	// MissingInstaller is the error message for not passing an installer in the request
	MissingInstaller = "installer info must be provided"
	// MissingUsernameError is the error message for not passing username in the request
	MissingUsernameError = "username must be provided"
	// ReservedUsernameError is the error message for passing a reserved username in the request
	ReservedUsernameError = "username is reserved"
	// MissingSSHKeyError is the error message when SSH Key is not given
	MissingSSHKeyError = "SSH key must be provided"
	// InvalidSSHKeyError is the error message for not supported or invalid ssh key format
	InvalidSSHKeyError = "SSH Key supports RSA or DSS or ED25519 or ECDSA-SHA2 algorithms"
)

//

// Required Packages to send to image builder that will go into the base image
var requiredPackages = []string{}

var (
	validSSHPrefix     = regexp.MustCompile(`^(ssh-(rsa|dss|ed25519)|ecdsa-sha2-nistp(256|384|521)) \S+`)
	validImageName     = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
	acceptedImageTypes = map[string]interface{}{ImageTypeCommit: nil, ImageTypeInstaller: nil}
)

var reservedImageUsernames = []string{
	"root", "bin", "daemon", "sys", "adm", "tty", "disk", "lp", "mem", "kmem", "wheel", "cdrom", "sync",
	"shutdown", "halt", "mail", "news", "uucp", "operator", "games", "gopher", "ftp", "man", "oprofile", "pkiuser",
	"dialout", "floppy", "games", "slocate", "utmp", "squid", "pvm", "named", "postgres", "mysql", "nscd",
	"rpcuser", "console", "rpc", "amandabackup", "tape", "netdump", "utempter", "vdsm", "kvm", "rpm", "ntp",
	"video", "dip", "mailman", "gdm", "xfs", "pppusers", "popusers", "slipusers", "mailnull", "apache", "wnn",
	"smmsp", "puppet", "tomcat", "lock", "ldap", "frontpage", "nut", "beagleindex", "tss", "piranha", "prelude-manager",
	"snortd", "audio", "condor", "nslcd", "wine", "pegasus", "webalizer", "haldaemon", "vcsa", "avahi",
	"realtime", "tcpdump", "privoxy", "sshd", "radvd", "cyrus", "saslauth", "arpwatch", "fax", "nocpulse", "desktop",
	"dbus", "jonas", "clamav", "screen", "quaggavt", "sabayon", "polkituser", "wbpriv", "postfix", "postdrop",
	"majordomo", "quagga", "exim", "distcache", "radiusd", "hsqldb", "dovecot", "ident", "users", "qemu",
	"ovirt", "rhevm", "jetty", "saned", "vhostmd", "usbmuxd", "bacula", "cimsrvr", "mock", "ricci",
	"luci", "activemq", "cassandra", "stap-server", "stapusr", "stapsys", "stapdev", "swift", "glance", "nova",
	"keystone", "quantum", "cinder", "ceilometer", "ceph", "avahi-autoipd", "pulse", "rtkit", "abrt",
	"retrace", "ovirtagent", "ats", "dhcpd", "myproxy", "sanlock", "aeolus", "wallaby", "katello", "elasticsearch",
	"mongodb", "wildfly", "jbosson-agent", "jbosson", "heat", "haproxy", "hacluster", "haclient", "systemd-journal",
	"systemd-network", "systemd-resolve", "gnats", "listar", "nobody",
}

func validateImageUserName(username string) error {
	if username == "" {
		return errors.New(MissingUsernameError)
	}
	for _, reservedName := range reservedImageUsernames {
		if strings.ToLower(username) == reservedName {
			return errors.New(ReservedUsernameError)
		}
	}
	return nil
}

// BeforeCreate method is called before creating ImageSet, it make sure org_id is not empty
func (imgset *ImageSet) BeforeCreate(tx *gorm.DB) error {
	if imgset.OrgID == "" {
		log.Error("imageSet do have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}

// ImageSetView is the image-set row returned for ui image-sets display
type ImageSetView struct {
	ID               uint        `json:"ID"`
	Name             string      `json:"Name"`
	Version          int         `json:"Version"`
	UpdatedAt        EdgeAPITime `json:"UpdatedAt"`
	Status           string      `json:"Status"`
	ImageBuildIsoURL string      `json:"ImageBuildIsoURL"`
	ImageID          uint        `json:"ImageID"`
}

// ImageView is the image row returned for ui images-set display
type ImageView struct {
	ID               uint        `json:"ID"`
	Name             string      `json:"Name"`
	Version          int         `json:"Version"`
	ImageType        string      `json:"ImageType"`
	CommitCheckSum   string      `json:"CommitCheckSum"`
	OutputTypes      []string    `json:"OutputTypes"`
	CreatedAt        EdgeAPITime `json:"CreatedAt"`
	Status           string      `json:"Status"`
	ImageBuildIsoURL string      `json:"ImageBuildIsoURL"`
}
