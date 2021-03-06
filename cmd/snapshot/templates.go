package main

import (
	"html/template"
	"log"
)

var (
	TT *template.Template
)

func init() {
	text := `
{{define "Diff"}}
<!DOCTYPE html>
<html>
  <head>
    {{template "Head"}}
    <style>
    td.value
    {
      max-width: 150px;
      word-wrap: break-word;
    }
    </style>
  </head>
  <body>
    <div class="container">
      <div class="row">
        <div class="col-xs-12">
          <h1>Differences</h1>
          <p><a href="/list">Back</a> | <a href="/export?ss1={{.From}}&ss2={{.To}}">Export</a></p>
          <table class="table">
            <col width="20%"/>
            <col width="20%"/>
            <col width="20%"/>
            <col width="20%"/>
            <col width="20%"/>
            <tr>
              <thead>
                <tr><th>path</th><th>scope</th><th>scope id</th><th class="value">old</th><th class="value">new</th></tr>
              </thead>
              <tbody>
                {{range .Lines}}
                <tr class="{{if .IsAdded}} success{{end}}{{if .IsRemoved}} danger{{end}}">
                  <td>{{.Path}}</td><td>{{.Scope}}</td><td>{{.ScopeId}}</td>
                  <td class="value">{{.OldValue}}</td>
                  <td class="value">{{.NewValue}}</td>
                </tr>
                {{end}}
              </tbody>
          </table>
          {{template "Foot"}}
        </div>
      </div>
    </div>
  </body>
</html>
{{end}}

{{define "List"}}
<!DOCTYPE html>
<html>
  <head>
    {{template "Head"}}
    <style>
    </style>
  </head>
  <body>
    <div class="container">

      <div class="row">
        <div class="col-xs-12">
          <h1>Snapshots</h1>
          <form action="take" method="post">
            <button type="submit" class="btn btn-default">Take Snapshot</button>
            <input type="text" name="message" />
          </form>
        </div>
      </div>

      <div class="row">
        <div class="col-xs-12">
          <form action="/diff" method="get" class="form">
            <table class="table">
              <tr>
                <thead>
                  <tr><th>1st</th><th>2nd</th><th>name</th><th>time</th><th>A</th><th>C</th><th>R</th></tr>
                </thead>
                <tbody>
                  {{range .Names}}
                  <tr>
                    <td><input type="radio" name="ss1" value="{{.N}}"/></td>
                    <td><input type="radio" name="ss2" value="{{.N}}"/></td>
                    <td>{{.Name}}</td>
                    <td>{{.Time}}</td>
                    <td class="{{if gt .Count.Added 0}}success{{end}}">{{.Count.Added}}</td>
                    <td class="{{if gt .Count.Changed 0}}info{{end}}">{{.Count.Changed}}</td>
                    <td class="{{if gt .Count.Removed 0}}danger{{end}}">{{.Count.Removed}}</td>
                  </tr>
                  {{end}}
                </tbody>
            </table>
            <button type="submit" class="btn btn-default">Diff</button>
          </form>
        </div>
      </div>
    </div>
    {{template "Foot"}}
  </body>
</html>
{{end}}

{{define "Head"}}
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css" integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap-theme.min.css" integrity="sha384-fLW2N01lMqjakBkx3l/M9EahuwpSfeNvV63J5ezn3uZzapT0u7EYsXMjQV+0En5r" crossorigin="anonymous">
{{end}}

{{define "Foot"}}
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/js/bootstrap.min.js" integrity="sha384-0mSbJDEHialfmuBBQP6A4Qrprq5OVfW37PRR3j5ELqxss1yVqOtnepnHVP9aJ7xS" crossorigin="anonymous"></script>
{{end}}
`
	var err error
	TT, err = template.New("TT").Parse(text)
	if err != nil {
		log.Fatal(err)
	}
}
