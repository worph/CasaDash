package apps

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// archiveDir zips srcDir into dstZip, preserving the leaf directory name as the
// top-level folder inside the archive (e.g. filebrowser/db/database.db).
func archiveDir(srcDir, dstZip string) error {
	zf, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	base := filepath.Dir(srcDir)
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}
