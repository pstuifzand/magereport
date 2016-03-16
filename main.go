package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"log"
	"os"
	"strings"
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

func main() {

	url, err := DatabaseConnectionString("app/etc/local.xml")
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", url)
	if err != nil {
		log.Fatal(err)
	}
	stateFilename := "config-report.json"

	firstRun := false
	oldVars, err := loadOldVars(stateFilename)
	if err != nil {
		firstRun = true
	}

	missing := make(map[string]bool)
	for k, _ := range oldVars {
		missing[k] = true
	}

	vars, err := DatabaseLoadConfig(db)
	if err != nil {
		log.Fatal(err)
	}

	for path, val := range vars {
		missing[path] = false
		if !firstRun {
			if oldVal, e := oldVars[path]; e {
				if oldVal != val {
					fmt.Printf("%s\n\told: %s\n\tnew: %s\n\n", path, oldVal, val)
				}
			} else {
				fmt.Printf("%s\n\tnew: %s\n\n", path, val)
			}
		}
	}
	for k, v := range missing {
		if v {
			fmt.Printf("%s\n\tis removed\n\told: %s\n\n", k, oldVars[k])
		}
	}

	err = saveOldVars(stateFilename, vars)
	if err != nil {
		log.Fatalf("Error: Can't open %s for writing: %s\n", stateFilename, err)
	}
}
