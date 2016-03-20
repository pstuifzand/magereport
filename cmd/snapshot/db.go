package main

import (
	"database/sql"
	"strings"
)

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
