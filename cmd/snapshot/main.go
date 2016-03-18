package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Connection struct {
	XMLName  xml.Name `xml:"connection"`
	Host     string   `xml:"host"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Dbname   string   `xml:"dbname"`
}

func (c Connection) String() string {
	return c.Username + ":" + c.Password + "@tcp(" + c.Host + ":3306)/" + c.Dbname
}

type DefaultSetup struct {
	XMLName    xml.Name `xml:"default_setup"`
	Connection Connection
}
type Resources struct {
	XMLName xml.Name `xml:"resources"`
	Setup   DefaultSetup
}

type Global struct {
	XMLName   xml.Name `xml:"global"`
	Resources Resources
}

type LocalXML struct {
	XMLName xml.Name `xml:"config"`
	Global  Global
}

func loadOldVars(filename string) (map[string]string, error) {
	input, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	r := json.NewDecoder(input)
	var vars map[string]string
	err = r.Decode(&vars)
	if err != nil {
		return nil, err
	}
	return vars, nil
}

func DatabaseConnectionString(filename string) (string, error) {
	xmlFile, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return "", err
	}
	defer xmlFile.Close()

	b, _ := ioutil.ReadAll(xmlFile)

	var q LocalXML

	xml.Unmarshal(b, &q)

	conn := q.Global.Resources.Setup.Connection

	return conn.String(), nil
}

func DatabaseLoadConfig(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT `scope`, `scope_id`, `path`, `value` FROM `core_config_data` ORDER BY `path`, `value`")
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	vars := make(map[string]string)
	for rows.Next() {
		var path, scope, scope_id string
		var value *string

		if err := rows.Scan(&scope, &scope_id, &path, &value); err != nil {
			return nil, err
		}
		var val string
		if value == nil {
			val = "<null>"
		} else {
			val = *value
		}

		path = strings.Join([]string{path, scope, scope_id}, "-")

		vars[path] = val
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return vars, nil
}

func saveOldVars(filename string, vars map[string]string) error {
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
	Db *sql.DB
}

func InitMagento(config string) (*Magento, error) {
	url, err := DatabaseConnectionString(config)
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
	magento.Db.Close()
}

func (magento *Magento) TakeSnapshot() error {
	t := time.Now()

	dir, err := os.Open(".snapshots")
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
	defer dir.Close()

	outputFilename := fmt.Sprintf(".snapshots/snapshot-%s.json", t.Format("2006-01-02_15-04-05"))

	db := magento.Db

	vars, err := DatabaseLoadConfig(db)
	if err != nil {
		return err
	}

	err = saveOldVars(outputFilename, vars)
	if err != nil {
		return err
	}

	return nil
}

func (magento *Magento) ListSnapshots() ([]Snapshot, error) {
	result := []Snapshot{}
	dir, err := os.Open(".snapshots")
	if err != nil {
		return result, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return result, err
	}
	sort.Strings(names)
	for i, filename := range names {
		if strings.HasPrefix(filename, "snapshot-") {
			result = append(result, Snapshot{i + 1, filename})
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
		fmt.Printf("% 4d\t%s\n", ss.N, ss.Name)
	}
	return nil
}

func (magento *Magento) LoadSnapshot(filename string) (map[string]string, error) {
	oldVars, err := loadOldVars(".snapshots/" + filename)
	return oldVars, err
}

type DiffLine struct {
	Path, OldValue, NewValue      string
	IsAdded, IsRemoved, IsChanged bool
	Scope                         string
	ScopeId                       int64
}

type DiffResult struct {
	Lines []DiffLine
}

var pathRegexp *regexp.Regexp

func MakeDiffLine(path, oldval, newval string) DiffLine {
	isAdded := newval != "" && oldval == ""
	isRemoved := newval == "" && oldval != ""
	isChanged := newval != "" && oldval != "" && oldval != newval
	parts := pathRegexp.FindStringSubmatch(path)
	scope := parts[2]
	scopeId, _ := strconv.ParseInt(parts[3], 10, 64)
	return DiffLine{parts[1], oldval, newval, isAdded, isRemoved, isChanged, scope, scopeId}
}

func (magento *Magento) Diff(snapshotFile1, snapshotFile2 string) (DiffResult, error) {
	oldVars, err := magento.LoadSnapshot(snapshotFile1)
	if err != nil {
		return DiffResult{}, err
	}

	missing := make(map[string]bool)
	for k, _ := range oldVars {
		missing[k] = true
	}

	vars, err := magento.LoadSnapshot(snapshotFile2)
	if err != nil {
		return DiffResult{}, err
	}

	paths := []string{}
	for k, _ := range vars {
		paths = append(paths, k)
	}

	sort.Strings(paths)

	result := DiffResult{}
	result.Lines = []DiffLine{}

	for _, path := range paths {
		val := vars[path]
		missing[path] = false
		if oldVal, e := oldVars[path]; e {
			if oldVal != val {
				result.Lines = append(result.Lines, MakeDiffLine(path, oldVal, val))
			}
		} else {
			result.Lines = append(result.Lines, MakeDiffLine(path, "", val))
		}
	}
	for k, v := range missing {
		if v {
			result.Lines = append(result.Lines, MakeDiffLine(k, oldVars[k], ""))
		}
	}

	return result, nil
}

var format *string

func init() {
	format = flag.String("format", "text", "format of output")
	pathRegexp = regexp.MustCompile("^(.*)-(default|websites|stores)-(\\d+)$")
}

type SnapshotHandler struct {
	Magento *Magento
}

func NewSnapshotHandler(magento *Magento) *SnapshotHandler {
	return &SnapshotHandler{magento}
}

type ListInfo struct {
	Names []Snapshot
}
type Snapshot struct {
	N    int
	Name string
}

func (snapshotHandler *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	magento := snapshotHandler.Magento
	if strings.HasPrefix("/take", r.URL.Path) {
		magento.TakeSnapshot()
		http.Redirect(w, r, "/list", 302)
		return
	} else if strings.HasPrefix("/list", r.URL.Path) {
		names, err := magento.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
		}
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(names)
		} else {
			templ, err := template.New("list.templ").ParseFiles(
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/list.templ",
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/head.templ",
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/foot.templ")

			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
			} else {
				err = templ.Execute(w, ListInfo{names})
				if err != nil {
					http.Error(w, fmt.Sprint(err), 500)
				}
			}
		}
		return
	} else if strings.HasPrefix("/diff", r.URL.Path) {
		names, err := magento.ListSnapshots()
		if err != nil {
			http.Error(w, fmt.Sprint(err), 500)
		}

		values := r.URL.Query()
		ss1, _ := strconv.ParseInt(values.Get("ss1"), 10, 32)
		ss2, _ := strconv.ParseInt(values.Get("ss2"), 10, 32)
		diff, err := magento.Diff(names[ss1-1].Name, names[ss2-1].Name)

		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(names)
		} else {
			templ, err := template.New("diff.templ").ParseFiles(
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/diff.templ",
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/head.templ",
				"/home/peter/work/go/src/github.com/pstuifzand/magereport/cmd/snapshot/foot.templ")

			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
			} else {
				err = templ.Execute(w, diff)
				if err != nil {
					http.Error(w, fmt.Sprint(err), 500)
				}
			}
		}
		return
	}
	http.NotFound(w, r)
}

func main() {
	flag.Parse()

	args := flag.Args()
	var cmd string
	if len(args) == 0 {
		cmd = "take"
	} else {
		cmd = args[0]
	}

	magento, err := InitMagento("app/etc/local.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer magento.Close()

	if cmd == "take" || cmd == "get" {
		magento.TakeSnapshot()
		if err != nil {
			log.Fatal(err)
		}
	} else if cmd == "list" {
		err = magento.List()
		if err != nil {
			log.Fatal(err)
		}
	} else if cmd == "diff" {
		names, err := magento.ListSnapshots()
		if err != nil {
			log.Fatal(err)
		}
		ss1, _ := strconv.ParseInt(args[1], 10, 32)
		ss2, _ := strconv.ParseInt(args[2], 10, 32)
		diff, err := magento.Diff(names[ss1-1].Name, names[ss2-1].Name)
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range diff.Lines {
			if r.OldValue == "" {
				fmt.Printf("%s\n\tnew: %s\n\n", r.Path, r.NewValue)
			} else if r.NewValue == "" {
				fmt.Printf("%s\n\tis removed\n\told: %s\n\n", r.Path, r.OldValue)
			} else {
				fmt.Printf("%s\n\told: %s\n\tnew: %s\n\n", r.Path, r.OldValue, r.NewValue)
			}
		}
	} else if cmd == "serve" {
		snapshotHandler := NewSnapshotHandler(magento)
		http.Handle("/", snapshotHandler)
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
