package pulp

import (
	"regexp"

	"github.com/google/uuid"
)

// 01902b07-242d-7ee0-9964-6191c8f8d622
var uuidRegexp = regexp.MustCompile("[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}")

func ScanUUID(href string) uuid.UUID {
	str := uuidRegexp.FindString(href)

	if str == "" {
		return uuid.UUID{}
	}

	u, err := uuid.Parse(str)
	if err != nil {
		return uuid.UUID{}
	}
	return u
}
