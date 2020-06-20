package leagues

import (
	"context"
	"github.com/leighmacdonald/steamid"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	lHist, err := parseRGL(b)
	require.NoError(t, err)
	require.Greater(t, len(lHist), 10)
}

func TestETF2L(t *testing.T) {
	c, cancel := context.WithTimeout(context.Background(), time.Second*25)
	defer cancel()
	seasons, err := getETF2L(c, 76561198004469267)
	require.NoError(t, err)
	require.Greater(t, len(seasons), 2)
}

func TestLogsTF(t *testing.T) {
	c, cancel := context.WithTimeout(context.Background(), time.Second*25)
	defer cancel()
	seasons, err := getLogsTF(c, 76561198086867244)
	require.NoError(t, err)
	require.Equal(t, 1, len(seasons))
	require.Greater(t, seasons[0].Count, 2100)
}

func TestFetchAll(t *testing.T) {
	c, cancel := context.WithTimeout(context.Background(), time.Second*25)
	defer cancel()
	seasons := FetchAll(c, steamid.SID64(76561197970669109))
	require.Greater(t, len(seasons), 20)
}
