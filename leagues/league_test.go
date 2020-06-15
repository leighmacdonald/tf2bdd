package leagues

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// exists checks for the existence of a file path
func exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// filePath will walk up the directory tree until it find a file. Max depth of 4
func filePath(p string) string {
	var dots []string
	for i := 0; i < 4; i++ {
		dir := path.Join(dots...)
		fPath := path.Join(dir, p)
		if exists(fPath) {
			fp, err := filepath.Abs(fPath)
			if err == nil {
				return fp
			}
			return fp
		}
		if strings.HasSuffix(dir, "tf2bdd") {
			return p
		}
		dots = append(dots, "..")
	}
	return p
}

func TestRGL(t *testing.T) {
	b, err := ioutil.ReadFile(filePath("examples/rgl.json"))
	require.NoError(t, err, "Could not open test file")
	var resp RGLResponse
	require.NoError(t, json.Unmarshal(b, &resp))
	require.NoError(t, parseRGLRank(&resp))
}
