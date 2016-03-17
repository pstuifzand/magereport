package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"log"
	"os"
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

func (magento *Magento) TakeSnapshot(outputFilename string) error {
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

func (magento *Magento) ListSnapshots() ([]string, error) {
	result := []string{}
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
	for _, filename := range names {
		if strings.HasPrefix(filename, "snapshot-") {
			result = append(result, filename)
		}
	}
	return result, nil
}

func (magento *Magento) List() error {
	names, err := magento.ListSnapshots()
	if err != nil {
		return err
	}
	for i, filename := range names {
		fmt.Printf("% 4d\t%s\n", i+1, filename)
	}
	return nil
}

func (magento *Magento) LoadSnapshot(filename string) (map[string]string, error) {
	oldVars, err := loadOldVars(".snapshots/" + filename)
	return oldVars, err
}

func (magento *Magento) Diff(snapshotFile1, snapshotFile2 string) error {
	log.Printf("Diff between %s and %s\n", snapshotFile1, snapshotFile2)
	oldVars, err := magento.LoadSnapshot(snapshotFile1)
	if err != nil {
		return err
	}

	missing := make(map[string]bool)
	for k, _ := range oldVars {
		missing[k] = true
	}

	vars, err := magento.LoadSnapshot(snapshotFile2)
	if err != nil {
		return err
	}

	paths := []string{}
	for k, _ := range vars {
		paths = append(paths, k)
	}

	sort.Strings(paths)

	for _, path := range paths {
		val := vars[path]
		missing[path] = false
		if oldVal, e := oldVars[path]; e {
			if oldVal != val {
				fmt.Printf("%s\n\told: %s\n\tnew: %s\n\n", path, oldVal, val)
			}
		} else {
			fmt.Printf("%s\n\tnew: %s\n\n", path, val)
		}
	}
	for k, v := range missing {
		if v {
			fmt.Printf("%s\n\tis removed\n\told: %s\n\n", k, oldVars[k])
		}
	}

	return nil
}

var format *string

func init() {
	format = flag.String("format", "text", "format of output")
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
		t := time.Now()

		dir, err := os.Open(".snapshots")
		if err != nil && os.IsNotExist(err) {
			log.Fatal(err)
		}
		defer dir.Close()

		err = magento.TakeSnapshot(fmt.Sprintf(".snapshots/snapshot-%s.json", t.Format("2006-01-02_15-04-05")))
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
		err = magento.Diff(names[ss1-1], names[ss2-1])
		if err != nil {
			log.Fatal(err)
		}
	}
}
