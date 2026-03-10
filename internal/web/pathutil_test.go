package web

import (
	"testing"
)

func TestSafePath(t *testing.T) {
	base := "/mnt/cam"

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid subdir", "TeslaCam/SavedClips", "/mnt/cam/TeslaCam/SavedClips", false},
		{"valid file", "TeslaCam/SavedClips/clip.mp4", "/mnt/cam/TeslaCam/SavedClips/clip.mp4", false},
		{"empty path returns base", "", "/mnt/cam", false},
		{"dot returns base", ".", "/mnt/cam", false},
		{"traversal blocked", "../../etc/passwd", "", true},
		{"nested traversal blocked", "TeslaCam/../../../etc/passwd", "", true},
		{"double dot in name OK", "TeslaCam/clip..v2.mp4", "/mnt/cam/TeslaCam/clip..v2.mp4", false},
		{"absolute path confined", "/etc/passwd", "/mnt/cam/etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safePath(base, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("safePath(%q, %q) = %q, want error", base, tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("safePath(%q, %q) error: %v", base, tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("safePath(%q, %q) = %q, want %q", base, tt.input, got, tt.want)
			}
		})
	}
}
