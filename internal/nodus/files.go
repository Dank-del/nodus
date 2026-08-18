package nodus

import (
	"fmt"
	"os"
	"path/filepath"
)

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".nodus-tmp-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func copyFile(source, destination string) error {
	b, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return atomicWrite(destination, b, 0o644)
}

func requireProject(root string) (Manifest, error) {
	m, err := LoadManifest(root)
	if err != nil {
		return Manifest{}, err
	}
	if m.Project.Backend != BackendCMake {
		return Manifest{}, fmt.Errorf("project backend %q is unavailable", m.Project.Backend)
	}
	return m, nil
}
