package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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
		diffRevs, err := GetDiffRevs(values.Get("ss1"), values.Get("ss2"), len(names))
		if err != nil {
			http.Error(w, fmt.Sprint(err), 400)
			return
		}

		diff, err := magento.Diff(names[diffRevs.Old].Name, names[diffRevs.New].Name)

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
