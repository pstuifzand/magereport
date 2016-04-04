package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pstuifzand/magereport/backend"
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

func loadOldVars(filename string) (backend.SnapshotVars, error) {
	input, err := os.Open(filename)
	if err != nil {
		return backend.SnapshotVars{}, err
	}
	defer input.Close()

	r := json.NewDecoder(input)
	var vars backend.SnapshotVars
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

func saveOldVars(filename string, vars backend.SnapshotVars) error {
	output, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer output.Close()
	w := json.NewEncoder(output)
	w.Encode(vars)
	return nil
}

type Magento struct {
	db *sql.DB
}

func (magento *Magento) createSnapshotDir() {
	dir, err := os.Open(".snapshots")
	if err != nil && os.IsNotExist(err) {
		os.Mkdir(".snapshots", 0755)
		return
	}
	defer dir.Close()
}

func InitMagento(configFilename string) (*Magento, error) {
	url, err := DatabaseConnectionString(configFilename)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, err
	}
	return &Magento{db}, nil
}

func (magento *Magento) Close() {
	magento.db.Close()
}

func (magento *Magento) TakeSnapshot(message string) error {
	t := time.Now().UTC()

	magento.createSnapshotDir()

	outputFilename := fmt.Sprintf(".snapshots/snapshot-%s.json", t.Format("2006-01-02_15-04-05"))

	db := magento.db

	vars, err := DatabaseLoadConfig(db)
	if err != nil {
		return err
	}
	fs := backend.SnapshotVars{message, vars}

	err = saveOldVars(outputFilename, fs)
	if err != nil {
		return err
	}

	return nil
}

func (magento *Magento) ListSnapshots() ([]backend.Snapshot, error) {
	result := []backend.Snapshot{}
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

			count := backend.DiffResultCount{0, 0, 0}
			if i >= 1 {
				prevFile := names[i-1]
				prevSnapshot, err := magento.LoadSnapshot(prevFile)
				curSnapshot, err := magento.LoadSnapshot(filename)

				diffResult, err := Diff(prevSnapshot, curSnapshot, i, i+1)
				if err == nil {
					count = diffResult.Count
				}
			}
			result = append(result, backend.Snapshot{i + 1, filename, count, tm})
		}
	}
	return result, nil
}

func (magento *Magento) LoadSnapshot(filename string) (backend.SnapshotVars, error) {
	oldVars, err := loadOldVars(".snapshots/" + filename)
	return oldVars, err
}
