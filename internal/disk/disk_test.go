package disk

import (
	"testing"
)

func TestExists(t *testing.T) {
	// On dev machine, backing file won't exist
	if Exists() {
		t.Skip("backing file exists, skipping")
	}
}

func TestCleanArtifacts(t *testing.T) {
	// Should not panic when mount point doesn't exist
	CleanArtifacts()
}
