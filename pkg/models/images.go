package models

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lib/pq"
)

// ImageSet represents a collection of images
type ImageSet struct {
	Model
	Name    string  `json:"Name"`
	Version int     `json:"Version" gorm:"default:1"`
	Account string  `json:"Account"`
	Images  []Image `json:"Images"`
}

// Image is what generates a OSTree Commit.
type Image struct {
	Model
	Name                   string           `json:"Name"`
	Account                string           `json:"Account"`
	OrgID                  string           `json:"org_id"`
	Distribution           string           `json:"Distribution"`
	Description            string           `json:"Description"`
	Status                 string           `json:"Status"`
	Version                int              `json:"Version" gorm:"default:1"`
	ImageType              string           `json:"ImageType"` // TODO: Remove as soon as the frontend stops using
	OutputTypes            pq.StringArray   `gorm:"type:text[]" json:"OutputTypes"`
	CommitID               uint             `json:"CommitID"`
	Commit                 *Commit          `json:"Commit"`
	InstallerID            *uint            `json:"InstallerID"`
	Installer              *Installer       `json:"Installer"`
	ImageSetID             *uint            `json:"ImageSetID" gorm:"index"` // TODO: Wipe staging database and set to not nullable
	Packages               []Package        `json:"Packages,omitempty" gorm:"many2many:images_packages;"`
	ThirdPartyRepositories []ThirdPartyRepo `json:"ThirdPartyRepositories,omitempty" gorm:"many2many:images_repos;"`
	CustomPackages         []Package        `json:"CustomPackages,omitempty" gorm:"many2many:images_custom_packages"`
	RequestID              string           `json:"request_id"` // storing for logging reference on resume
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

// Required Packages to send to image builder that will go into the base image
var requiredPackages = [6]string{
	"ansible",
	"rhc",
	"rhc-worker-playbook",
	"subscription-manager",
	"subscription-manager-plugin-ostree",
	"insights-client",
}

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

// ValidateRequest validates an Image Request
func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New(DistributionCantBeNilMessage)
	}
	if !validImageName.MatchString(i.Name) {
		return errors.New(NameCantBeInvalidMessage)
	}
	if i.Commit == nil || i.Commit.Arch == "" {
		return errors.New(ArchitectureCantBeEmptyMessage)
	}
	if len(i.OutputTypes) == 0 {
		return errors.New(NoOutputTypes)
	}
	for _, out := range i.OutputTypes {
		if _, ok := acceptedImageTypes[out]; !ok {
			return errors.New(ImageTypeNotAccepted)
		}
	}
	// Installer checks
	if i.HasOutputType(ImageTypeInstaller) {
		if i.Installer == nil {
			return errors.New(MissingInstaller)
		}
		if err := validateImageUserName(i.Installer.Username); err != nil {
			return err
		}
		if i.Installer.SSHKey == "" {
			return errors.New(MissingSSHKeyError)
		}
		if !validSSHPrefix.MatchString(i.Installer.SSHKey) {
			return errors.New(InvalidSSHKeyError)
		}

	}
	return nil
}

// HasOutputType checks if an image has an specific output type
func (i *Image) HasOutputType(imageType string) bool {
	for _, out := range i.OutputTypes {
		if out == imageType {
			return true
		}
	}
	return false
}

// GetPackagesList returns the packages in a user-friendly list containing their names
func (i *Image) GetPackagesList() *[]string {
	l := len(requiredPackages)
	pkgs := make([]string, len(i.Packages)+l)
	for i, p := range requiredPackages {
		pkgs[i] = p
	}
	for i, p := range i.Packages {
		pkgs[i+l] = p.Name
	}
	return &pkgs
}

// GetALLPackagesList returns all the packages including custom packages containing their names
func (i *Image) GetALLPackagesList() *[]string {
	initialPackages := *i.GetPackagesList()
	packages := make([]string, 0, len(initialPackages)+len(i.CustomPackages))

	packages = append(packages, initialPackages...)

	for _, pkg := range i.CustomPackages {
		packages = append(packages, pkg.Name)
	}
	return &packages
}
