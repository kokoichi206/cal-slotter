package update

import "testing"

func TestAssetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		goos    string
		goarch  string
		want    string
	}{
		{
			name:    "macOS arm64",
			version: "0.1.0",
			goos:    "darwin",
			goarch:  "arm64",
			want:    "slotter_0.1.0_darwin_arm64.tar.gz",
		},
		{
			name:    "Linux amd64",
			version: "0.1.0",
			goos:    "linux",
			goarch:  "amd64",
			want:    "slotter_0.1.0_linux_amd64.tar.gz",
		},
		{
			name:    "Windows amd64",
			version: "0.1.0",
			goos:    "windows",
			goarch:  "amd64",
			want:    "slotter_0.1.0_windows_amd64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AssetName(tt.version, tt.goos, tt.goarch)
			if got != tt.want {
				t.Fatalf("AssetName() = %q, want %q", got, tt.want)
			}
		})
	}
}
