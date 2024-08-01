package pulp

import (
	"regexp"
	"strconv"

	"github.com/google/uuid"
)

// 01902b07-242d-7ee0-9964-6191c8f8d622
var uuidRegexp = regexp.MustCompile("[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}")

func ScanUUID(href *string) uuid.UUID {
	if href == nil {
		return uuid.UUID{}
	}

	str := uuidRegexp.FindString(*href)

	if str == "" {
		return uuid.UUID{}
	}

	u, err := uuid.Parse(str)
	if err != nil {
		return uuid.UUID{}
	}
	return u
}

// /api/pulp/edge-integration-test-2/api/v3/repositories/file/file/01910e45-ceb3-7213-bed8-0727e76d0d12/versions/1/
var repoVerRegexp = regexp.MustCompile("versions/([0-9]+)")

func ScanRepoFileVersion(href *string) int64 {
	if href == nil {
		return 0
	}

	str := repoVerRegexp.FindStringSubmatch(*href)

	if len(str) != 2 {
		return 0
	}

	result, err := strconv.Atoi(str[1])
	if err != nil {
		return 0
	}

	return int64(result)
}
