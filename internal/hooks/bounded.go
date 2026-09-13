package hooks

import (
	"io"
	"os"
)

// readBounded reads at most limit+1 bytes so a huge file cannot flood the context.
func readBounded(path string, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
