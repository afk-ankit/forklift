package cmd

import "testing"

func TestDesktopCopy(t *testing.T) {
	tests := []struct {
		event       string
		wantTitle   string
		wantMessage string
	}{
		{"started", "Forklift Build Started", "Tag v1.2.3 workflow started."},
		{"success", "Forklift Build Complete", "Tag v1.2.3 built successfully!"},
		{"failure", "Forklift Build Failed", "Tag v1.2.3 build failed."},
		{"cancelled", "Forklift Build Cancelled", "Tag v1.2.3 build was cancelled."},
		{"unknown", "Forklift", "Tag v1.2.3: unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			gotTitle, gotMessage := desktopCopy(tt.event, "v1.2.3")
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotMessage != tt.wantMessage {
				t.Errorf("message = %q, want %q", gotMessage, tt.wantMessage)
			}
		})
	}
}
