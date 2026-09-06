// Package localfiles holds development-only credentials outside source control.
package localfiles

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func Read(path string, value any) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() > 64<<10 {
		return errors.New("local credential file must be private and regular")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
func Write(path string, value any, replace bool) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return WriteBytes(path, raw, replace)
}
func WriteBytes(path string, raw []byte, replace bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return errors.New("local credentials directory must be private")
	}
	target := path
	if replace {
		target = path + ".next"
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if replace {
		return os.Rename(target, path)
	}
	return nil
}
