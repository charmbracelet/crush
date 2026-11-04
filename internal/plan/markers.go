// Package plan contains the markers used to identify completed plans.
package plan

import "strings"

// StartMarker marks the beginning of a final plan response.
const StartMarker = "<!-- CRUSH_PLAN_START -->"

// ReadyMarker marks a plan that is ready for execution.
const ReadyMarker = "<!-- CRUSH_PLAN_READY -->"

// StartMarkerPresent reports whether the start marker is on its own line.
func StartMarkerPresent(text string) bool {
	return markerPresent(text, StartMarker)
}

// ReadyMarkerPresent reports whether the ready marker is on its own line.
func ReadyMarkerPresent(text string) bool {
	return markerPresent(text, ReadyMarker)
}

func markerPresent(text, marker string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if markerLine(line, marker) {
			return true
		}
	}
	return false
}

func markerLine(line, marker string) bool {
	trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`"))
	return trimmed == marker
}

// StripReadyMarker removes ready marker lines and their empty code fences.
func StripReadyMarker(text string) string {
	return stripMarker(text, ReadyMarker)
}

// StripMarkers removes both plan markers from text.
func StripMarkers(text string) string {
	return stripMarker(stripMarker(text, StartMarker), ReadyMarker)
}

func stripMarker(text, marker string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	inFence := false
	for i := 0; i < len(lines); i++ {
		if markerLine(lines[i], marker) {
			// Drop a fence pair wrapping only the marker.
			if inFence && len(kept) > 0 && isCodeFenceLine(kept[len(kept)-1]) &&
				i+1 < len(lines) && isCodeFenceLine(lines[i+1]) {
				kept = kept[:len(kept)-1]
				i++
				inFence = false
			}
			continue
		}
		if isCodeFenceLine(lines[i]) {
			inFence = !inFence
		}
		kept = append(kept, lines[i])
	}
	return strings.Join(kept, "\n")
}

func isCodeFenceLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return false
	}
	info := strings.TrimLeft(trimmed[3:], "`")
	return !strings.ContainsAny(info, "` ")
}
