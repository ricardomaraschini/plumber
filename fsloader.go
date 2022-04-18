package plumber

import (
	"embed"
	"fmt"
	"path"

	"sigs.k8s.io/kustomize/api/filesys"
)

// LoadFS loads an embed FS struct into an in memory kustomize file system representation. Reads
// all files from the embed and writes them to the FileSystem struct.
func LoadFS(content embed.FS) (filesys.FileSystem, error) {
	fs := filesys.MakeFsInMemory()
	if err := readdir(".", content, fs); err != nil {
		return nil, fmt.Errorf("error loading embed files: %w", err)
	}
	return fs, nil
}

// readdir reads a directory recursively from provided embed instance, copying everything into
// a fs.FileSystem object. Any error aborts the process and 'to' is left in an unknown state.
func readdir(dir string, from embed.FS, to filesys.FileSystem) error {
	entries, err := from.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("error reading dir: %w", err)
	}

	for _, entry := range entries {
		path := path.Join(dir, entry.Name())

		if entry.IsDir() {
			if err := readdir(path, from, to); err != nil {
				return err
			}
			continue
		}

		fcontent, err := from.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		if err := to.WriteFile(path, fcontent); err != nil {
			return fmt.Errorf("error writing file: %w", err)
		}
	}
	return nil
}
