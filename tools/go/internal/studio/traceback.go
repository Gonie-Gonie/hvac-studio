package studio

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type sourceLocation struct {
	ComponentID string
	Source      string
	Line        int
}

type tracebackFrame struct {
	Path string
	Line int
}

func tracebackSourceLocation(loaded *project.LoadedProject, message string, preferredComponentID string) (sourceLocation, bool) {
	frames := tracebackFrames(message)
	if len(frames) == 0 {
		return sourceLocation{}, false
	}
	paths := componentEditableSourcePaths(loaded)
	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		for _, candidate := range paths {
			if preferredComponentID != "" && candidate.ComponentID != preferredComponentID {
				continue
			}
			if sameTracebackPath(frame.Path, candidate.AbsPath) {
				return sourceLocation{
					ComponentID: candidate.ComponentID,
					Source:      candidate.Source,
					Line:        frame.Line,
				}, true
			}
		}
	}
	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		for _, candidate := range paths {
			if sameTracebackPath(frame.Path, candidate.AbsPath) {
				return sourceLocation{
					ComponentID: candidate.ComponentID,
					Source:      candidate.Source,
					Line:        frame.Line,
				}, true
			}
		}
	}
	return tracebackProjectSourceLocation(loaded, frames, preferredComponentID)
}

func tracebackFrames(message string) []tracebackFrame {
	pattern := regexp.MustCompile(`(?m)^\s*File "([^"]+)", line ([0-9]+)`)
	matches := pattern.FindAllStringSubmatch(message, -1)
	frames := make([]tracebackFrame, 0, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		line, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		frames = append(frames, tracebackFrame{Path: filepath.Clean(match[1]), Line: line})
	}
	return frames
}

type componentSourcePathCandidate struct {
	ComponentID string
	Source      string
	AbsPath     string
}

func componentEditableSourcePaths(loaded *project.LoadedProject) []componentSourcePathCandidate {
	paths := []componentSourcePathCandidate{}
	for _, component := range loaded.Graph.Components {
		sourcePath, err := componentSourcePath(loaded, component.ID)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(loaded.Root, sourcePath)
		if err != nil {
			rel = sourcePath
		}
		paths = append(paths, componentSourcePathCandidate{
			ComponentID: component.ID,
			Source:      filepath.ToSlash(rel),
			AbsPath:     filepath.Clean(sourcePath),
		})
	}
	return paths
}

func tracebackProjectSourceLocation(loaded *project.LoadedProject, frames []tracebackFrame, preferredComponentID string) (sourceLocation, bool) {
	absRoot, err := filepath.Abs(loaded.Root)
	if err != nil {
		return sourceLocation{}, false
	}
	absRoot = canonicalExistingPath(absRoot)

	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		framePath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(frame.Path)))
		if framePath == "" {
			continue
		}
		if !filepath.IsAbs(framePath) {
			framePath = filepath.Join(absRoot, framePath)
		}
		frameAbs, err := filepath.Abs(framePath)
		if err != nil {
			continue
		}
		frameAbs = canonicalExistingPath(frameAbs)
		rel, err := filepath.Rel(absRoot, frameAbs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return sourceLocation{
			ComponentID: preferredComponentID,
			Source:      filepath.ToSlash(rel),
			Line:        frame.Line,
		}, true
	}
	return sourceLocation{}, false
}

func sameTracebackPath(tracebackPath string, sourcePath string) bool {
	tracebackPath = cleanPathForComparison(tracebackPath)
	sourcePath = cleanPathForComparison(sourcePath)
	if filepath.IsAbs(tracebackPath) {
		if tracebackAbs, err := filepath.Abs(tracebackPath); err == nil {
			tracebackPath = tracebackAbs
		}
	}
	if sourceAbs, err := filepath.Abs(sourcePath); err == nil {
		sourcePath = sourceAbs
	}
	if sameExistingFile(tracebackPath, sourcePath) {
		return true
	}
	tracebackPath = canonicalExistingPath(tracebackPath)
	sourcePath = canonicalExistingPath(sourcePath)
	if strings.EqualFold(tracebackPath, sourcePath) {
		return true
	}
	tracebackSlash := filepath.ToSlash(tracebackPath)
	sourceSlash := filepath.ToSlash(sourcePath)
	return strings.HasSuffix(sourceSlash, tracebackSlash)
}

func cleanPathForComparison(path string) string {
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
}

func canonicalExistingPath(path string) string {
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		path = evaluated
	}
	return filepath.Clean(path)
}

func sameExistingFile(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}
