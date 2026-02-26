package archive

import (
	"testing"
)

func TestIsReachableNoConfig(t *testing.T) {
	// With no config loaded, should return false
	if IsReachable() {
		t.Error("expected false with no config")
	}
}
