package artifactschema

import "testing"

func TestReportAcceptsCurrentAlphaPatchLine(t *testing.T) {
	report, err := Report("project", "0.1.7")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Compatible || report.NeedsMigration {
		t.Fatalf("report = %#v", report)
	}
}

func TestCheckRejectsDifferentMinorVersion(t *testing.T) {
	err := Check("graph", "0.2.0")
	if err == nil {
		t.Fatal("expected incompatible schema error")
	}
}

func TestReportRejectsInvalidVersion(t *testing.T) {
	_, err := Report("project", "alpha")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
}
