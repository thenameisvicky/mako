package storage

import (
	"fmt"
	"strings"
)

type QueryBuilder struct {
	table     string
	action    string // SELECT, INSERT, UPDATE
	columns   []string
	values    []interface{}
	where     []string
	whereArgs []interface{}
	set       []string
	setArgs   []interface{}
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.action = "SELECT"
	qb.columns = columns
	return qb
}

func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.table = table
	return qb
}

func (qb *QueryBuilder) InsertInto(table string) *QueryBuilder {
	qb.action = "INSERT"
	qb.table = table
	return qb
}

func (qb *QueryBuilder) Columns(columns ...string) *QueryBuilder {
	qb.columns = columns
	return qb
}

func (qb *QueryBuilder) Values(values ...interface{}) *QueryBuilder {
	qb.values = values
	return qb
}

func (qb *QueryBuilder) Update(table string) *QueryBuilder {
	qb.action = "UPDATE"
	qb.table = table
	return qb
}

func (qb *QueryBuilder) Set(column string, val interface{}) *QueryBuilder {
	qb.set = append(qb.set, column)
	qb.setArgs = append(qb.setArgs, val)
	return qb
}

func (qb *QueryBuilder) Where(column string, val interface{}) *QueryBuilder {
	qb.where = append(qb.where, column)
	qb.whereArgs = append(qb.whereArgs, val)
	return qb
}

func (qb *QueryBuilder) Build() (string, []interface{}, error) {
	switch qb.action {
	case "SELECT":
		var query strings.Builder
		query.WriteString("SELECT ")
		query.WriteString(strings.Join(qb.columns, ", "))
		query.WriteString(" FROM ")
		query.WriteString(qb.table)

		var args []interface{}
		if len(qb.where) > 0 {
			query.WriteString(" WHERE ")
			var conditions []string
			for i, col := range qb.where {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", col, i+1))
				args = append(args, qb.whereArgs[i])
			}
			query.WriteString(strings.Join(conditions, " AND "))
		}
		return query.String(), args, nil

	case "INSERT":
		var query strings.Builder
		query.WriteString("INSERT INTO ")
		query.WriteString(qb.table)
		query.WriteString(" (")
		query.WriteString(strings.Join(qb.columns, ", "))
		query.WriteString(") VALUES (")
		
		var placeholders []string
		for i := 1; i <= len(qb.values); i++ {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		}
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
		return query.String(), qb.values, nil

	case "UPDATE":
		var query strings.Builder
		query.WriteString("UPDATE ")
		query.WriteString(qb.table)
		query.WriteString(" SET ")

		var args []interface{}
		var setClauses []string
		idx := 1
		for i, col := range qb.set {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, idx))
			args = append(args, qb.setArgs[i])
			idx++
		}
		query.WriteString(strings.Join(setClauses, ", "))

		if len(qb.where) > 0 {
			query.WriteString(" WHERE ")
			var conditions []string
			for i, col := range qb.where {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", col, idx))
				args = append(args, qb.whereArgs[i])
				idx++
			}
			query.WriteString(strings.Join(conditions, " AND "))
		}
		return query.String(), args, nil

	default:
		return "", nil, fmt.Errorf("unknown action: %s", qb.action)
	}
}
