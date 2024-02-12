package owdb

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// fs.go provides the backend layer for persistence of a [Store]. The functions
// in this file are mostly called internally by methods of Store.

// createFileBackup makes a duplicate of file in the same location with '.bak'
// appended to its filename. file must be an orbweaver data file. Any existing
// backup is overwritten.
//
// returns path to new backup file and any error that occurred.
func createFileBackup(file string) (string, error) {
	backupDir := filepath.Dir(file)
	backupName := filepath.Base(file) + ".bak"

	buPath := filepath.Join(backupDir, backupName)

	// underlying io
	rf, err := os.Open(file)
	if err != nil {
		return buPath, fmt.Errorf("open original: %w", err)
	}
	defer rf.Close()
	wf, err := os.Create(buPath)
	if err != nil {
		return buPath, fmt.Errorf("create backup: %w", err)
	}
	defer wf.Close()

	// buffered io
	r := bufio.NewReader(rf)
	w := bufio.NewWriter(wf)

	_, err = io.Copy(w, r)
	if err != nil {
		return buPath, fmt.Errorf("copy data to backup: %w", err)
	}

	return buPath, nil
}
