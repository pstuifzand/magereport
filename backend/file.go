package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var fileRegexp *regexp.Regexp

func init() {
	fileRegexp = regexp.MustCompile(`^snapshot-(\d{4}-\d{2}-\d{2})_(\d{2}-\d{2}-\d{2}).json$`)
}

type FileBackend struct {
	source SourceBackend
}

func InitFileBackend(source SourceBackend) Backend {
	return &FileBackend{source}
}

func (this *FileBackend) TakeSnapshot(message string) error {
	vars, err := this.source.TakeSnapshot(message)
	if err != nil {
		return err
	}
	err = this.SaveSnapshot(vars)
	return err
}

func loadOldVars(filename string) (SnapshotVars, error) {
	input, err := os.Open(filename)
	if err != nil {
		return SnapshotVars{}, err
	}
	defer input.Close()

	r := json.NewDecoder(input)
	var vars SnapshotVars
	err = r.Decode(&vars)
	if err != nil || len(vars.Vars) == 0 {
		if err != nil {
			// Couldn't decode the new file version
			// try the old version
			log.Println(err)
		}
		input.Seek(0, 0)
		r = json.NewDecoder(input)
		var configVars map[string]string
		err = r.Decode(&configVars)
		vars.Vars = configVars
		vars.Message = ""
		return vars, nil
	}
	return vars, nil
}

func saveOldVars(filename string, vars SnapshotVars) error {
	output, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer output.Close()
	w := json.NewEncoder(output)
	w.Encode(vars)
	return nil
}

func (this *FileBackend) SaveSnapshot(vars SnapshotVars) error {
	t := time.Now().UTC()

	createSnapshotDir()
	outputFilename := fmt.Sprintf(".snapshots/snapshot-%s.json", t.Format("2006-01-02_15-04-05"))

	err := saveOldVars(outputFilename, vars)
	if err != nil {
		return err
	}

	return nil
}

func createSnapshotDir() {
	dir, err := os.Open(".snapshots")
	if err != nil && os.IsNotExist(err) {
		os.Mkdir(".snapshots", 0755)
		return
	}
	defer dir.Close()
}

func (this *FileBackend) ListSnapshots() ([]Snapshot, error) {
	result := []Snapshot{}
	dir, err := os.Open(".snapshots")
	if err != nil {
		// no results, because not dir (not an error)
		return result, nil
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return result, err
	}
	sort.Strings(names)
	for i, filename := range names {
		if strings.HasPrefix(filename, "snapshot-") {
			parts := fileRegexp.FindStringSubmatch(filename)
			d := parts[1]
			t := parts[2]

			tm, err := time.Parse("2006-01-02 15-04-05", d+" "+t)
			if err != nil {
				tm = time.Now().UTC()
			}

			count := DiffResultCount{0, 0, 0}
			if i >= 1 {
				prevFile := names[i-1]
				prevSnapshot, err := this.LoadSnapshot(prevFile)
				curSnapshot, err := this.LoadSnapshot(filename)

				diffResult, err := Diff(prevSnapshot, curSnapshot, i, i+1)
				if err == nil {
					count = diffResult.Count
				}
			}
			result = append(result, Snapshot{i + 1, filename, count, tm})
		}
	}
	return result, nil
}

func (this *FileBackend) LoadSnapshot(filename string) (SnapshotVars, error) {
	oldVars, err := loadOldVars(".snapshots/" + filename)
	return oldVars, err
}

func (this *FileBackend) Close() {
}
