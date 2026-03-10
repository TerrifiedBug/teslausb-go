package web

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const mountTestTimeout = 5 * time.Second

type mountTest struct {
	host    string
	port    string
	source  string                          // e.g. "server:/share" or "//server/share"
	testDir string                          // temp mount point
	mount   func(dir string) ([]byte, error) // execute the mount command
	cleanup func()                          // optional extra cleanup (e.g. remove credentials file)
}

func (s *Server) runMountTest(w http.ResponseWriter, mt mountTest) {
	// TCP connectivity check
	conn, err := net.DialTimeout("tcp", mt.host+":"+mt.port, mountTestTimeout)
	if err != nil {
		jsonResponse(w, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Cannot reach %s:%s — %v", mt.host, mt.port, err),
		})
		return
	}
	conn.Close()

	// Temp mount
	os.MkdirAll(mt.testDir, 0755)
	defer func() {
		exec.Command("umount", "-f", "-l", mt.testDir).Run()
		os.Remove(mt.testDir)
		if mt.cleanup != nil {
			mt.cleanup()
		}
	}()

	out, err := mt.mount(mt.testDir)
	if err != nil {
		jsonResponse(w, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Mount failed: %s", strings.TrimSpace(string(out))),
		})
		return
	}

	jsonResponse(w, map[string]any{
		"ok":      true,
		"message": fmt.Sprintf("Successfully mounted %s", mt.source),
	})
}
