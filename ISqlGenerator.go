package gSqlHelper

type ISqlGenerator interface {
	GeneratorAutoMigrationSql(values ...interface{}) (string, error)
	// Tables
	generatorCreateTableSql(dst ...interface{}) (string, error)
	// Columns
	generateAddColumnSql(dst interface{}, field string) (string, error)
	// Constraints
	generateCreateConstraint(dst interface{}, name string) (string, error)
	// Indexes
	generateCreateIndex(dst interface{}, name string) (string, error)
}
