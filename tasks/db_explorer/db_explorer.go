package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type Response map[string]interface{}

type Column struct {
	Name     string
	Type     string
	Nullable bool
	Primary  bool
}

type Table struct {
	Name       string
	Columns    []Column
	PrimaryKey string
}

func __err_panic(err error) {
	if err != nil {
		panic(err)
	}
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	tables := []Table{}

	query := "SHOW TABLES"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		table := Table{}
		rows.Scan(&table.Name)
		tables = append(tables, table)
	}

	for i := range tables {
		query := "SHOW FULL COLUMNS FROM " + tables[i].Name
		rows, err := db.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, err
		}

		rValues := make([]reflect.Value, len(columnTypes))
		iValues := make([]interface{}, len(columnTypes))

		for i := range columnTypes {
			rValues[i] = reflect.New(columnTypes[i].ScanType())
			iValues[i] = rValues[i].Interface()
		}

		for rows.Next() {
			rows.Scan(iValues...)
			column := Column{}
			for j := range columnTypes {
				if columnTypes[j].Name() == "Field" {
					rb := rValues[j].Elem().Interface().(sql.RawBytes)
					column.Name = string(rb)
				}
				if columnTypes[j].Name() == "Type" {
					rb := rValues[j].Elem().Interface().(sql.RawBytes)
					if strings.Split(string(rb), "(")[0] == "varchar" || string(rb) == "text" {
						column.Type = "string"
					}
					if string(rb) == "int" {
						column.Type = "int"
					}
				}
				if columnTypes[j].Name() == "Null" {
					rb := rValues[j].Elem().Interface().(sql.RawBytes)
					if string(rb) == "YES" {
						column.Nullable = true
					}
				}
				if columnTypes[j].Name() == "Key" {
					rb := rValues[j].Elem().Interface().(sql.RawBytes)
					if string(rb) == "PRI" {
						tables[i].PrimaryKey = column.Name
					}
				}
			}
			tables[i].Columns = append(tables[i].Columns, column)
		}
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foundTable := func(tableName string) (Table, bool) {
			for _, table := range tables {
				if table.Name == tableName {
					return table, true
				}
			}
			response := Response{
				"error": "unknown table",
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.WriteHeader(http.StatusNotFound)
			w.Write(json)
			return Table{}, false
		}

		splitPath := strings.Split(r.URL.Path, "/")[1:]

		switch {
		case r.Method == "GET" && len(splitPath) == 1 && splitPath[0] == "":
			tableNames := []string{}
			for _, table := range tables {
				tableNames = append(tableNames, table.Name)
			}

			response := Response{
				"response": Response{
					"tables": tableNames,
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)

		case r.Method == "GET" && len(splitPath) == 1 && splitPath[0] != "":
			_table, found := foundTable(splitPath[0])
			if !found {
				return
			}

			query := "SELECT * FROM " + _table.Name

			rows, err := db.Query(query)
			__err_panic(err)
			defer rows.Close()

			rValues := make([]reflect.Value, len(_table.Columns))
			iValues := make([]interface{}, len(_table.Columns))

			for i := range _table.Columns {
				if _table.Columns[i].Type == "string" {
					rValues[i] = reflect.New(reflect.PtrTo(reflect.TypeOf("")))
				}
				if _table.Columns[i].Type == "int" {
					rValues[i] = reflect.New(reflect.PtrTo(reflect.TypeOf(0)))
				}
				iValues[i] = rValues[i].Interface()
			}

			records := []map[string]interface{}{}
			for rows.Next() {
				rows.Scan(iValues...)
				record := make(map[string]interface{}, len(_table.Columns))
				for i := range _table.Columns {
					if rv := rValues[i].Elem(); rv.IsNil() {
						record[_table.Columns[i].Name] = nil
					} else {
						record[_table.Columns[i].Name] = rv.Elem().Interface()
					}
				}
				records = append(records, record)
			}

			limit, err := strconv.Atoi(r.FormValue("limit"))
			if err != nil {
				limit = 5
			}
			offset, err := strconv.Atoi(r.FormValue("offset"))
			if err != nil {
				offset = 0
			}

			lb := offset
			rb := offset + limit
			if lb > len(records) {
				lb = len(records)
			}
			if rb > len(records) {
				rb = len(records)
			}

			response := Response{
				"response": Response{
					"records": records[lb:rb],
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)

		case r.Method == "GET" && len(splitPath) == 2:
			_table, found := foundTable(splitPath[0])
			if !found {
				return
			}

			id, err := strconv.Atoi(splitPath[1])
			__err_panic(err)

			query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", _table.Name, _table.PrimaryKey)

			row := db.QueryRow(query, id)

			rValues := make([]reflect.Value, len(_table.Columns))
			iValues := make([]interface{}, len(_table.Columns))

			for i := range _table.Columns {
				if _table.Columns[i].Type == "string" {
					rValues[i] = reflect.New(reflect.PtrTo(reflect.TypeOf("")))
				}
				if _table.Columns[i].Type == "int" {
					rValues[i] = reflect.New(reflect.PtrTo(reflect.TypeOf(0)))
				}
				iValues[i] = rValues[i].Interface()
			}

			err = row.Scan(iValues...)
			if err != nil {
				if err == sql.ErrNoRows {
					response := Response{
						"error": "record not found",
					}
					json, err := json.Marshal(response)
					__err_panic(err)
					w.WriteHeader(http.StatusNotFound)
					w.Write(json)
				} else {
					panic(err)
				}
				return
			}

			record := make(map[string]interface{}, len(_table.Columns))
			for i := range _table.Columns {
				if rv := rValues[i].Elem(); rv.IsNil() {
					record[_table.Columns[i].Name] = nil
				} else {
					record[_table.Columns[i].Name] = rv.Elem().Interface()
				}
			}

			response := Response{
				"response": Response{
					"record": record,
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)

		case r.Method == "PUT" && len(splitPath) == 2 && splitPath[1] == "":
			_table, found := foundTable(splitPath[0])
			if !found {
				return
			}

			defer r.Body.Close()
			body, err := ioutil.ReadAll(r.Body)
			__err_panic(err)

			values := map[string]interface{}{}
			err = json.Unmarshal(body, &values)
			__err_panic(err)

			query := fmt.Sprintf("INSERT INTO %s (", _table.Name)
			arguments := []interface{}{}
			for _, column := range _table.Columns {
				if column.Name == _table.PrimaryKey {
					continue
				}
				query = query + fmt.Sprintf("%s, ", column.Name)
				if value, ok := values[column.Name]; ok {
					arguments = append(arguments, value)
				} else {
					if column.Nullable {
						arguments = append(arguments, nil)
					} else {
						if column.Type == "string" {
							arguments = append(arguments, "")
						} else if column.Type == "int" {
							arguments = append(arguments, 0)
						} else {
							panic("unknown column type")
						}
					}

				}
			}
			query = query[0:len(query)-2] + ") VALUES ("
			for i := 0; i < len(arguments); i++ {
				query = query + "?, "
			}
			query = query[0:len(query)-2] + ")"

			result, err := db.Exec(query, arguments...)
			__err_panic(err)

			lastId, err := result.LastInsertId()
			__err_panic(err)

			response := Response{
				"response": Response{
					_table.PrimaryKey: lastId,
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)

		case r.Method == "POST" && len(splitPath) == 2:
			_table, found := foundTable(splitPath[0])
			if !found {
				return
			}

			id, err := strconv.Atoi(splitPath[1])
			__err_panic(err)

			defer r.Body.Close()
			body, err := ioutil.ReadAll(r.Body)
			__err_panic(err)

			values := map[string]interface{}{}
			err = json.Unmarshal(body, &values)
			__err_panic(err)

			query := fmt.Sprintf("UPDATE %s SET ", _table.Name)
			arguments := []interface{}{}
			for _, column := range _table.Columns {
				if value, ok := values[column.Name]; ok {
					valid := true
					if column.Name == _table.PrimaryKey {
						valid = false
					}
					if value == nil && !column.Nullable {
						valid = false
					}
					if value != nil {
						if column.Type == "string" {
							if _, ok := value.(string); !ok {
								valid = false
							}
						}
						if column.Type == "int" {
							if f, ok := value.(float64); !ok {
								valid = false
							} else {
								_, d := math.Modf(f)
								if d != float64(0) {
									valid = false
								}
							}
						}
					}
					if !valid {
						response := Response{
							"error": fmt.Sprintf("field %s have invalid type", column.Name),
						}
						json, err := json.Marshal(response)
						__err_panic(err)
						w.WriteHeader(http.StatusBadRequest)
						w.Write(json)
						return
					}
					query = query + fmt.Sprintf("%s = ?, ", column.Name)
					arguments = append(arguments, value)
				}
			}
			query = query[0:len(query)-2] + fmt.Sprintf(" WHERE %s = ?", _table.PrimaryKey)
			arguments = append(arguments, id)

			result, err := db.Exec(query, arguments...)
			__err_panic(err)

			affected, err := result.RowsAffected()
			__err_panic(err)

			response := Response{
				"response": Response{
					"updated": affected,
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)

		case r.Method == "DELETE" && len(splitPath) == 2:
			_table, found := foundTable(splitPath[0])
			if !found {
				return
			}

			id, err := strconv.Atoi(splitPath[1])
			__err_panic(err)

			query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", _table.Name, _table.PrimaryKey)

			result, err := db.Exec(query, id)
			__err_panic(err)

			affected, err := result.RowsAffected()
			__err_panic(err)

			response := Response{
				"response": Response{
					"deleted": affected,
				},
			}
			json, err := json.Marshal(response)
			__err_panic(err)
			w.Write(json)
		}
	})

	return h, nil
}
