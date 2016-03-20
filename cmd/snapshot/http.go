package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type SnapshotHandler struct {
	Magento *Magento
}

func NewSnapshotHandler(magento *Magento) *SnapshotHandler {
	return &SnapshotHandler{magento}
}

type ListInfo struct {
	Names []Snapshot
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

			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
			} else {
				err = TT.ExecuteTemplate(w, "List", ListInfo{names})
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
			return
		}

		values := r.URL.Query()
		ss1, err := strconv.ParseInt(values.Get("ss1"), 10, 32)
		if err != nil {
			http.Error(w, "Argument not a number", 400)
			return
		}
		ss2, err := strconv.ParseInt(values.Get("ss2"), 10, 32)
		if err != nil {
			http.Error(w, "Argument not a number", 400)
			return
		}
		ss1 -= 1
		ss2 -= 1
		if (ss1 < 0 && int(ss1) >= len(names)) || ss2 < 0 && int(ss2) >= len(names) {
			http.Error(w, "Argument out of range", 400)
			return
		}
		diff, err := magento.Diff(names[ss1].Name, names[ss2].Name)

		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			w.Header().Add("Content-Type", "application/json")
			json.NewEncoder(w).Encode(names)
		} else {
			if err != nil {
				http.Error(w, fmt.Sprint(err), 500)
			} else {
				err = TT.ExecuteTemplate(w, "Diff", diff)
				if err != nil {
					http.Error(w, fmt.Sprint(err), 500)
				}
			}
		}
		return
	}
	http.NotFound(w, r)
}
