// Package other writes whatever path it is handed; it knows nothing about git.
package other

import "os"

// Write writes path.
func Write(path string) error { return os.WriteFile(path, nil, 0o600) }
