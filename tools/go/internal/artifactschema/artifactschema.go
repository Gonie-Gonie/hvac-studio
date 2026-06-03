package artifactschema

import (
	"fmt"
	"strconv"
	"strings"
)

const CurrentVersion = "0.1.0"

type Compatibility struct {
	Kind           string `json:"kind"`
	Version        string `json:"version"`
	CurrentVersion string `json:"current_version"`
	Compatible     bool   `json:"compatible"`
	NeedsMigration bool   `json:"needs_migration"`
	Policy         string `json:"policy"`
}

func Check(kind string, version string) error {
	report, err := Report(kind, version)
	if err != nil {
		return err
	}
	if !report.Compatible {
		return fmt.Errorf("%s schema_version %s is not compatible with %s", kind, version, CurrentVersion)
	}
	return nil
}

func Report(kind string, version string) (Compatibility, error) {
	version = strings.TrimSpace(version)
	report := Compatibility{
		Kind:           strings.TrimSpace(kind),
		Version:        version,
		CurrentVersion: CurrentVersion,
		Policy:         "alpha 0.1.x artifacts are load-compatible; other major/minor versions require migration",
	}
	if version == "" {
		return report, fmt.Errorf("%s schema_version is required", kind)
	}
	currentMajor, currentMinor, err := majorMinor(CurrentVersion)
	if err != nil {
		return report, err
	}
	major, minor, err := majorMinor(version)
	if err != nil {
		return report, fmt.Errorf("%s schema_version is invalid: %s", kind, version)
	}
	report.Compatible = major == currentMajor && minor == currentMinor
	report.NeedsMigration = !report.Compatible
	return report, nil
}

func majorMinor(version string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("version must include major and minor: %s", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}
