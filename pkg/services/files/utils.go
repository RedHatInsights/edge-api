// FIXME: golangci-lint
// nolint:revive
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const CloudwatchMaximumLogMessageSize = 262144

func sanitizePath(destination string, filePath string) (destpath string, err error) {
	destpath = filepath.Join(destination, filePath)
	prefix := filepath.Clean(destination + string(os.PathSeparator))
	if !strings.HasPrefix(destpath, prefix) {
		err = fmt.Errorf("%s: illegal file path, prefix: %s, destpath: %s", filePath, prefix, destpath)
	}
	return
}

// objectIsWithinMemoryLimit returns a bool result from compare an object with Cloudwatch Maximum Log Message Size
func objectIsWithinMemoryLimit(object interface{}) bool {
	objReflection := reflect.ValueOf(object)
	len, _ := Sizeof(objReflection)
	return len < CloudwatchMaximumLogMessageSize
}

// Sizeof receive a reflect value and returns the length and capacity of an object in memory using in Kb
func Sizeof(rv reflect.Value) (int, int) {
	rt := rv.Type()
	if rt.Kind() == reflect.Slice {
		size := int(rt.Size())
		if rv.Len() > 0 {
			l, c := Sizeof(rv.Index(0))
			return size + (l * rv.Len()), size + (c * rv.Cap())
		}

	}
	return int(rt.Size()), int(rt.Size())
}
