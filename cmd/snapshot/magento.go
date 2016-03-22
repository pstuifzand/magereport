package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var pathRegexp *regexp.Regexp
var fileRegexp *regexp.Regexp

func init() {
	pathRegexp = regexp.MustCompile("^(.*)-(default|websites|stores)-(\\d+)$")
	fileRegexp = regexp.MustCompile(`^snapshot-(\d{4}-\d{2}-\d{2})_(\d{2}-\d{2}-\d{2}).json$`)
}

type Snapshot struct {
	N     int
	Name  string
	Count DiffResultCount
	Time  time.Time
}

func (self *DiffResultCount) Changes() int {
	return self.Added + self.Removed + self.Changed
}

func (self *DiffResultCount) String() string {
	return fmt.Sprintf("A%d C%d R%d",
		self.Added, self.Removed, self.Changed)
}

func (this *Snapshot) String() string {
	return fmt.Sprintf("% 4d %-20s %s %v", this.N, this.Name, this.Count.String(), this.Time)
}

type FileSnapshot struct {
	Message string
	Vars    map[string]string
}

func loadOldVars(filename string) (FileSnapshot, error) {
	input, err := os.Open(filename)
	if err != nil {
		return FileSnapshot{}, err
	}
	defer input.Close()

	r := json.NewDecoder(input)
	var vars FileSnapshot
	err = r.Decode(&vars)
	if err != nil || len(vars.Vars) == 0 {
		if err != nil {
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

func saveOldVars(filename string, vars FileSnapshot) error {
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
	fs := FileSnapshot{message, vars}

	err = saveOldVars(outputFilename, fs)
	if err != nil {
		return err
	}

	return nil
}

func (magento *Magento) ListSnapshots() ([]Snapshot, error) {
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
				diffResult, err := magento.Diff(prevFile, filename, i, i+1)
				if err == nil {
					count = diffResult.Count
				}
			}
			result = append(result, Snapshot{i + 1, filename, count, tm})
		}
	}
	return result, nil
}

func (magento *Magento) List() error {
	names, err := magento.ListSnapshots()
	if err != nil {
		return err
	}
	for _, ss := range names {
		fmt.Printf("%s\n", ss.String())
		//fmt.Printf("% 4d\t%s\n", ss.N, ss.Name)
	}
	return nil
}

func (magento *Magento) LoadSnapshot(filename string) (FileSnapshot, error) {
	oldVars, err := loadOldVars(".snapshots/" + filename)
	return oldVars, err
}

type DiffLine struct {
	Path, OldValue, NewValue      string
	IsAdded, IsRemoved, IsChanged bool
	Scope                         string
	ScopeId                       int64
}

type DiffResultCount struct {
	Added, Changed, Removed int
}

type DiffResult struct {
	Lines    []DiffLine
	Count    DiffResultCount
	From, To int
}

func makeDiffLine(path, oldval, newval string) DiffLine {
	isAdded := newval != "" && oldval == ""
	isRemoved := newval == "" && oldval != ""
	isChanged := newval != "" && oldval != "" && oldval != newval
	parts := pathRegexp.FindStringSubmatch(path)
	scope := parts[2]
	scopeId, _ := strconv.ParseInt(parts[3], 10, 64)
	return DiffLine{parts[1], oldval, newval, isAdded, isRemoved, isChanged, scope, scopeId}
}

func (magento *Magento) Diff(snapshotFile1, snapshotFile2 string, from, to int) (DiffResult, error) {
	oldSnapshot, err := magento.LoadSnapshot(snapshotFile1)
	if err != nil {
		return DiffResult{}, err
	}

	missing := make(map[string]bool)
	for k, _ := range oldSnapshot.Vars {
		missing[k] = true
	}

	newSnapshot, err := magento.LoadSnapshot(snapshotFile2)
	if err != nil {
		return DiffResult{}, err
	}

	paths := []string{}
	for k, _ := range newSnapshot.Vars {
		paths = append(paths, k)
	}

	sort.Strings(paths)

	result := DiffResult{}
	result.Lines = []DiffLine{}
	result.From = from + 1
	result.To = to + 1

	count := DiffResultCount{0, 0, 0}

	for _, path := range paths {
		val := newSnapshot.Vars[path]
		missing[path] = false
		if oldVal, e := oldSnapshot.Vars[path]; e {
			if oldVal != val {
				result.Lines = append(result.Lines, makeDiffLine(path, oldVal, val))
				count.Changed += 1
			}
		} else {
			result.Lines = append(result.Lines, makeDiffLine(path, "", val))
			count.Added += 1
		}
	}
	for k, v := range missing {
		if v {
			result.Lines = append(result.Lines, makeDiffLine(k, oldSnapshot.Vars[k], ""))
			count.Removed += 1
		}
	}

	result.Count = count

	return result, nil
}
