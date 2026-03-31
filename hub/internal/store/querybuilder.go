package store

import (
	"strings"
)

const sqlWhere = " WHERE "

type queryBuilder struct {
	base     string
	wheres   []string
	args     []interface{}
	orderBy  string
	limit    int
	offset   int
	hasLimit bool
	hasOff   bool
}

func newQueryBuilder(base string) *queryBuilder {
	return &queryBuilder{base: base}
}

func (qb *queryBuilder) Where(condition string, args ...interface{}) {
	qb.wheres = append(qb.wheres, condition)
	qb.args = append(qb.args, args...)
}

func (qb *queryBuilder) WhereIn(column string, values []interface{}) {
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	qb.wheres = append(qb.wheres, column+" IN ("+strings.Join(placeholders, ",")+")")
	qb.args = append(qb.args, values...)
}

func (qb *queryBuilder) OrderBy(clause string) {
	qb.orderBy = clause
}

func (qb *queryBuilder) Limit(n int) {
	qb.limit = n
	qb.hasLimit = true
}

func (qb *queryBuilder) Offset(n int) {
	qb.offset = n
	qb.hasOff = true
}

func (qb *queryBuilder) Build() (string, []interface{}) {
	q := qb.base
	args := make([]interface{}, len(qb.args))
	copy(args, qb.args)
	if len(qb.wheres) > 0 {
		q += sqlWhere + strings.Join(qb.wheres, " AND ")
	}
	if qb.orderBy != "" {
		q += " ORDER BY " + qb.orderBy
	}
	if qb.hasLimit {
		q += " LIMIT ?"
		args = append(args, qb.limit)
	}
	if qb.hasOff {
		q += " OFFSET ?"
		args = append(args, qb.offset)
	}
	return q, args
}

func (qb *queryBuilder) CountQuery(table string) (string, []interface{}) {
	q := "SELECT COUNT(*) FROM " + table
	if len(qb.wheres) > 0 {
		q += sqlWhere + strings.Join(qb.wheres, " AND ")
	}
	return q, qb.args
}

// BuildWhereClause returns just the WHERE clause (including the "WHERE" keyword)
// and the args. Returns an empty string if there are no conditions.
func (qb *queryBuilder) BuildWhereClause() (string, []interface{}) {
	if len(qb.wheres) == 0 {
		return "", qb.args
	}
	return sqlWhere + strings.Join(qb.wheres, " AND "), qb.args
}
