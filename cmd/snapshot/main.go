package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

var format *string
var host *string
var port *int

func init() {
	format = flag.String("format", "text", "format of output")
	port = flag.Int("port", 8080, "port")
	host = flag.String("host", "0.0.0.0", "host")
}

func main() {
	flag.Parse()

	args := flag.Args()
	var cmd string
	if len(args) == 0 {
		cmd = "help"
	} else {
		cmd = args[0]
	}

	magento, err := InitMagento("app/etc/local.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer magento.Close()

	if cmd == "take" || cmd == "get" {
		err = magento.TakeSnapshot()
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
	} else if cmd == "help" {
		fmt.Print(`Take config snapshots and show differences
Usage: snapshot [command] [options]

Commands:
serve     serves the snapshots in a webserver
take      takes a snapshots of the current configuration in the database
list      lists snapshots for current dir
diff A B  show the differences between snapshot A and snapshot B [diff 1 3]
help      this list
`)
	} else if cmd == "serve" {
		snapshotHandler := NewSnapshotHandler(magento)
		http.Handle("/", snapshotHandler)

		url := fmt.Sprintf("%s:%d", *host, *port)
		log.Printf("Snapshot server is hosted at http://%s/\n", url)
		log.Fatal(http.ListenAndServe(url, nil))
	}
}
