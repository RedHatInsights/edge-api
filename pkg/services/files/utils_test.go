// FIXME: golangci-lint
// nolint:revive
package files

import "testing"

func TestSanitizeWithFilePathSameAsDest(t *testing.T) {
	path, err := sanitizePath("/tmp/repos/5535", "./")
	if path != "/tmp/repos/5535" {
		t.Error(path)
	}
	if err != nil {
		t.Error(err)
	}
}
func TestSanitizeWithFilePathWithFolder(t *testing.T) {
	path, err := sanitizePath("/tmp/repos/5535", "abc")
	if path != "/tmp/repos/5535/abc" {
		t.Error(path)
	}
	if err != nil {
		t.Error(err)
	}
}

func TestObjectIsWithinMemoryLimitPositiveReturn(t *testing.T) {
	result := objectIsWithinMemoryLimit(make([][256]byte, 8, 16))
	if !result {
		t.Error("object is within the limit")
	}
}

func TestStringIsWithinMemoryLimitPositiveReturn(t *testing.T) {
	result := objectIsWithinMemoryLimit("stringTest")
	if !result {
		t.Error("object is within the limit")
	}
}
func TestMapIsWithinMemoryLimitPositiveReturn(t *testing.T) {
	mapTest := make(map[string]int)
	mapTest["k1"] = 1
	result := objectIsWithinMemoryLimit(mapTest)
	if !result {
		t.Error("object is within the limit")
	}
}
func TestObjectIsWithinMemoryLimitNegativeReturn(t *testing.T) {
	result := objectIsWithinMemoryLimit(make([][271890]byte, 8, 16))
	if result {
		t.Error("object is exceeding the limit")
	}
}
