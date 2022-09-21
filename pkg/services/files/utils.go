// FIXME: golangci-lint
// nolint:revive
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func sanitizePath(destination string, filePath string) (destpath string, err error) {
	destpath = filepath.Join(destination, filePath)
	prefix := filepath.Clean(destination + string(os.PathSeparator))
	if !strings.HasPrefix(destpath, prefix) {
		err = fmt.Errorf("%s: illegal file path, prefix: %s, destpath: %s", filePath, prefix, destpath)
	}
	return
}
