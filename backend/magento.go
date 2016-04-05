package backend

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"os"
	"strings"
)

type magentoBackend struct {
	db *sql.DB
}

type Connection struct {
	XMLName  xml.Name `xml:"connection"`
	Host     string   `xml:"host"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Dbname   string   `xml:"dbname"`
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

func (c Connection) String() string {
	return c.Username + ":" + c.Password + "@tcp(" + c.Host + ":3306)/" + c.Dbname
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
	if conn.Host == "localhost" {
		sockname := "/var/run/mysqld/mysqld.sock"
		_, err = os.Open(sockname)
		if !os.IsNotExist(err) {
			return conn.Username + ":" + conn.Password + "@unix(" + sockname + ")/" + conn.Dbname, nil
		}
	}
	return conn.String(), nil
}

func databaseLoadConfig(db *sql.DB) (map[string]string, error) {
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
			val = ""
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

func InitMagento(configFilename string) (SourceBackend, error) {
	url, err := DatabaseConnectionString(configFilename)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, err
	}
	return &magentoBackend{db}, nil
}

func (magento *magentoBackend) TakeSnapshot(message string) (SnapshotVars, error) {
	db := magento.db

	vars, err := databaseLoadConfig(db)
	if err != nil {
		return SnapshotVars{}, err
	}
	fs := SnapshotVars{message, vars}

	return fs, nil
}

func (magento *magentoBackend) Close() {
	magento.db.Close()
}
