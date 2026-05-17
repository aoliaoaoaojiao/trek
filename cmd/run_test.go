package cmd

import "testing"

func TestResolveArtifactDir(t *testing.T) {
	tests := []struct {
		name        string
		reportFile  string
		artifactDir string
		want        string
	}{
		{name: "explicit artifact dir", reportFile: "log/report.json", artifactDir: "log/raw", want: "log/raw"},
		{name: "derive from report json", reportFile: "log/report.json", want: "log/report_artifacts"},
		{name: "derive from report md", reportFile: "log/report.md", want: "log/report_artifacts"},
		{name: "empty", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveArtifactDir(tt.reportFile, tt.artifactDir)
			if got != tt.want {
				t.Fatalf("resolveArtifactDir 错误: got=%q want=%q", got, tt.want)
			}
		})
	}
}
