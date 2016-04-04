package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/pstuifzand/magereport/backend"
	"log"
	"net/http"
	"os"
	"strconv"
)

var format *string
var host *string
var port *int
var message *string

func init() {
	format = flag.String("format", "text", "format of output")
	port = flag.Int("port", 8080, "port")
	host = flag.String("host", "0.0.0.0", "host")
	message = flag.String("m", "", "add a message to the snapshot")
}

type DiffRevs struct {
	Old, New int
}

func GetDiffRevs(oldref, newref string, maxCount int) (DiffRevs, error) {
	ss1, err := strconv.ParseInt(oldref, 10, 32)
	if err != nil {
		return DiffRevs{}, err
	}
	ss2, err := strconv.ParseInt(newref, 10, 32)
	if err != nil {
		return DiffRevs{}, err
	}
	ss1 -= 1
	ss2 -= 1
	if (ss1 < 0 && int(ss1) >= maxCount) || (ss2 < 0 && int(ss2) >= maxCount) {
		return DiffRevs{}, errors.New("Argument is out of range")
	}
	return DiffRevs{int(ss1), int(ss2)}, nil
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

	magento, err := backend.InitMagento("app/etc/local.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer magento.Close()

	fb := backend.InitFileBackend(magento)

	if cmd == "take" || cmd == "get" {
		msg := ""
		msg = *message
		err := fb.TakeSnapshot(msg)
		if err != nil {
			log.Fatal(err)
		}
	} else if cmd == "list" {
		names, err := fb.ListSnapshots()
		if err != nil {
			log.Fatal(err)
		}
		for _, ss := range names {
			fmt.Printf("%s\n", ss.String())
		}
	} else if cmd == "export" {
		names, err := fb.ListSnapshots()
		if err != nil {
			log.Fatal(err)
		}
		diffRevs, err := GetDiffRevs(args[1], args[2], len(names))
		oldSnapshot, err := fb.LoadSnapshot(names[diffRevs.Old].Name)
		if err != nil {
			log.Fatal(err)
		}
		newSnapshot, err := fb.LoadSnapshot(names[diffRevs.New].Name)
		if err != nil {
			log.Fatal(err)
		}

		err = backend.Export(os.Stdout, oldSnapshot, newSnapshot, diffRevs.Old, diffRevs.New)
		if err != nil {
			log.Fatal(err)
		}

	} else if cmd == "diff" {
		names, err := fb.ListSnapshots()
		if err != nil {
			log.Fatal(err)
		}
		diffRevs, err := GetDiffRevs(args[1], args[2], len(names))

		oldSnapshot, err := fb.LoadSnapshot(names[diffRevs.Old].Name)
		if err != nil {
			log.Fatal(err)
		}
		newSnapshot, err := fb.LoadSnapshot(names[diffRevs.New].Name)
		if err != nil {
			log.Fatal(err)
		}

		diff, err := backend.Diff(oldSnapshot, newSnapshot, diffRevs.Old, diffRevs.New)
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
serve       serves the snapshots in a webserver
take        takes a snapshots of the current configuration in the database
list        lists snapshots for current dir
diff A B    show the differences between snapshot A and snapshot B [diff 1 3]
export A B  show the differences between snapshot A and snapshot B [export 1 3]
            and exports the difference to magerun format "config:set"
help        this list

Options:
-port=port  port for serve command
-host=host  host for serve command
`)
	} else if cmd == "serve" {
		snapshotHandler := NewSnapshotHandler(fb)
		http.Handle("/", snapshotHandler)

		url := fmt.Sprintf("%s:%d", *host, *port)
		log.Printf("Snapshot server is hosted at http://%s/\n", url)
		log.Fatal(http.ListenAndServe(url, nil))
	}
}
