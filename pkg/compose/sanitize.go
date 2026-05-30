package compose

import "strings"

// SanitizeVolumes filters out banned bind mounts from a volume list.
// Returns the sanitized list and a list of removed volumes.
func SanitizeVolumes(volumes []string) (sanitized []string, removed []string) {
	for _, vol := range volumes {
		if isBannedVolume(vol) {
			removed = append(removed, vol)
		} else {
			sanitized = append(sanitized, vol)
		}
	}
	return sanitized, removed
}

// isBannedVolume returns true if the volume is a bind mount targeting /home/agent.
func isBannedVolume(vol string) bool {
	if !isBindMount(vol) {
		return false
	}
	parts := strings.SplitN(vol, ":", 2)
	if len(parts) < 2 {
		return false
	}
	target := parts[1]
	// Remove :ro, :rw suffixes if present
	if idx := strings.Index(target, ":"); idx != -1 {
		target = target[:idx]
	}
	return target == "/home/agent" || strings.HasPrefix(target, "/home/agent/")
}

// isBindMount returns true if the volume spec is a bind mount (host path, not named volume).
// Bind mounts start with '.', '/', or '~'.
func isBindMount(vol string) bool {
	if vol == "" {
		return false
	}
	parts := strings.SplitN(vol, ":", 2)
	source := parts[0]
	return strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~")
}
