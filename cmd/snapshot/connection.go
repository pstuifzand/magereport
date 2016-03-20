package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
)

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

	return conn.String(), nil
}
