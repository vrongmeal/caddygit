package utils

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ErrInvalidPath is returned when the path does not exist.
var ErrInvalidPath = errors.New("filepath does not exist")

// IsDir tells if the path is a directory or not. It returns ErrInvalidPath
// if the path does not exist.
func IsDir(root string) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ErrInvalidPath
		}

		return false, err
	}

	if !info.IsDir() {
		return false, nil
	}

	return true, nil
}

// IsDirEmpty tells if the directory path is empty or not.
func IsDirEmpty(root string) (bool, error) {
	f, err := os.Open(filepath.Clean(root))
	if err != nil {
		return false, err
	}
	defer f.Close() // nolint:errcheck

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}
