package gSqlHelper

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gm "gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
	"strings"
)

type MySqlGenerator struct {
	gm.Migrator
}

func NewMyGenerator(db *gorm.DB) *MySqlGenerator {
	newMyGenerator := &MySqlGenerator{}
	newMyGenerator.DB = db
	newMyGenerator.Dialector = db.Dialector
	return newMyGenerator
}

func (mg *MySqlGenerator) GeneratorAutoMigrationSql(values ...interface{}) (string, error) {
	autoMigrationSqlStr := ""

	for _, value := range mg.ReorderModels(values, true) {
		tx := mg.DB.Session(&gorm.Session{})
		if !tx.Migrator().HasTable(value) {
			if sql, err := mg.generatorCreateTableSql(value); err != nil { //输出建表sql
				return "", err
			} else {
				autoMigrationSqlStr += sql + ";"
			}
		} else {
			if err := mg.RunWithValue(value, func(stmt *gorm.Statement) (err error) {
				columnTypes, _ := mg.DB.Migrator().ColumnTypes(value) //返回表字段类型数组

				for _, dbName := range stmt.Schema.DBNames {
					//field := stmt.Schema.FieldsByDBName[dbName]
					var foundColumn gorm.ColumnType

					for _, columnType := range columnTypes {
						if columnType.Name() == dbName {
							foundColumn = columnType
							break
						}
					}

					if foundColumn == nil {
						// not found, add column
						sql, err := mg.generateAddColumnSql(value, dbName)
						if err != nil {
							return err
						}
						autoMigrationSqlStr += sql + ";"
					}

				}

				for _, rel := range stmt.Schema.Relationships.Relations {
					if !mg.DB.Config.DisableForeignKeyConstraintWhenMigrating {
						if constraint := rel.ParseConstraint(); constraint != nil &&
							constraint.Schema == stmt.Schema && !tx.Migrator().HasConstraint(value, constraint.Name) {
							sql, err := mg.generateCreateConstraint(value, constraint.Name)
							if err != nil {
								return err
							}
							autoMigrationSqlStr += sql + ";"
						}
					}

					for _, chk := range stmt.Schema.ParseCheckConstraints() {
						if !tx.Migrator().HasConstraint(value, chk.Name) {
							sql, err := mg.generateCreateConstraint(value, chk.Name)
							if err != nil {
								return err
							}
							autoMigrationSqlStr += sql + ";"
						}
					}
				}

				for _, idx := range stmt.Schema.ParseIndexes() {
					if !tx.Migrator().HasIndex(value, idx.Name) {
						sql, err := mg.generateCreateIndex(value, idx.Name)
						if err != nil {
							return err
						}
						autoMigrationSqlStr += sql + ";"
					}
				}

				return nil
			}); err != nil {
				return "", err
			}
		}
	}

	return autoMigrationSqlStr, nil
}

func (mg *MySqlGenerator) generatorCreateTableSql(values ...interface{}) (string, error) {
	var sqlStr string
	for _, value := range mg.ReorderModels(values, false) {
		tx := mg.DB.Session(&gorm.Session{})
		if err := mg.RunWithValue(value, func(stmt *gorm.Statement) (errr error) {
			var (
				createTableSQL          = "CREATE TABLE ? ("
				values                  = []interface{}{mg.CurrentTable(stmt)}
				hasPrimaryKeyInDataType bool
			)

			for _, dbName := range stmt.Schema.DBNames {
				field := stmt.Schema.FieldsByDBName[dbName]
				if !field.IgnoreMigration {
					createTableSQL += "? ?"
					hasPrimaryKeyInDataType = hasPrimaryKeyInDataType || strings.Contains(strings.ToUpper(string(field.DataType)), "PRIMARY KEY")
					values = append(values, clause.Column{Name: dbName}, mg.DB.Migrator().FullDataTypeOf(field))
					createTableSQL += ","
				}
			}

			if !hasPrimaryKeyInDataType && len(stmt.Schema.PrimaryFields) > 0 {
				createTableSQL += "PRIMARY KEY ?,"
				primaryKeys := []interface{}{}
				for _, field := range stmt.Schema.PrimaryFields {
					primaryKeys = append(primaryKeys, clause.Column{Name: field.DBName})
				}

				values = append(values, primaryKeys)
			}

			for _, idx := range stmt.Schema.ParseIndexes() {
				if mg.CreateIndexAfterCreateTable {
					defer func(value interface{}, name string) {
						if errr == nil {
							errr = tx.Migrator().CreateIndex(value, name)
						}
					}(value, idx.Name)
				} else {
					if idx.Class != "" {
						createTableSQL += idx.Class + " "
					}
					createTableSQL += "INDEX ? ?"

					if idx.Comment != "" {
						createTableSQL += fmt.Sprintf(" COMMENT '%s'", idx.Comment)
					}

					if idx.Option != "" {
						createTableSQL += " " + idx.Option
					}

					createTableSQL += ","
					values = append(values, clause.Expr{SQL: idx.Name}, tx.Migrator().(gm.BuildIndexOptionsInterface).BuildIndexOptions(idx.Fields, stmt))
				}
			}

			for _, rel := range stmt.Schema.Relationships.Relations {
				if !mg.DB.DisableForeignKeyConstraintWhenMigrating {
					if constraint := rel.ParseConstraint(); constraint != nil {
						if constraint.Schema == stmt.Schema {
							sql, vars := buildConstraint(constraint)
							createTableSQL += sql + ","
							values = append(values, vars...)
						}
					}
				}
			}

			for _, chk := range stmt.Schema.ParseCheckConstraints() {
				createTableSQL += "CONSTRAINT ? CHECK (?),"
				values = append(values, clause.Column{Name: chk.Name}, clause.Expr{SQL: chk.Constraint})
			}

			createTableSQL = strings.TrimSuffix(createTableSQL, ",")

			createTableSQL += ")"

			if tableOption, ok := mg.DB.Get("gorm:table_options"); ok {
				createTableSQL += fmt.Sprint(tableOption)
			}

			sqlStr = mg.PrintSql(createTableSQL, values...)
			return errr
		}); err != nil {
			return "", err
		}
	}
	return sqlStr, nil
}

func (mg *MySqlGenerator) generateAddColumnSql(value interface{}, name string) (string, error) {
	var sqlStr string

	err := mg.RunWithValue(value, func(stmt *gorm.Statement) error {
		// avoid using the same name field
		f := stmt.Schema.LookUpField(name)
		if f == nil {
			return fmt.Errorf("failed to look up field with name: %s", name)
		}
		sqlStr = mg.PrintSql(
			"ALTER TABLE ? ADD ? ?",
			mg.CurrentTable(stmt), clause.Column{Name: f.DBName}, mg.DB.Migrator().FullDataTypeOf(f),
		)

		if !f.IgnoreMigration {
			return nil
		}

		return nil
	})
	return sqlStr, err
}

func (mg *MySqlGenerator) generateCreateConstraint(value interface{}, name string) (string, error) {
	var sqlStr string
	err := mg.RunWithValue(value, func(stmt *gorm.Statement) error {
		constraint, chk, table := mg.GuessConstraintAndTable(stmt, name)
		if chk != nil {
			sqlStr = mg.PrintSql(
				"ALTER TABLE ? ADD CONSTRAINT ? CHECK (?)",
				mg.CurrentTable(stmt), clause.Column{Name: chk.Name}, clause.Expr{SQL: chk.Constraint},
			)

			return nil
		}

		if constraint != nil {
			vars := []interface{}{clause.Table{Name: table}}
			if stmt.TableExpr != nil {
				vars[0] = stmt.TableExpr
			}
			sql, values := buildConstraint(constraint)
			sqlStr = mg.PrintSql("ALTER TABLE ? ADD "+sql, append(vars, values...)...)
			return nil
		}
		return nil
	})
	return sqlStr, err
}

func (mg *MySqlGenerator) generateCreateIndex(value interface{}, name string) (string, error) {
	var sqlStr string
	err := mg.RunWithValue(value, func(stmt *gorm.Statement) error {
		if idx := stmt.Schema.LookIndex(name); idx != nil {
			opts := mg.DB.Migrator().(gm.BuildIndexOptionsInterface).BuildIndexOptions(idx.Fields, stmt)
			values := []interface{}{clause.Column{Name: idx.Name}, mg.CurrentTable(stmt), opts}

			createIndexSQL := "CREATE "
			if idx.Class != "" {
				createIndexSQL += idx.Class + " "
			}
			createIndexSQL += "INDEX ? ON ??"

			if idx.Type != "" {
				createIndexSQL += " USING " + idx.Type
			}

			if idx.Comment != "" {
				createIndexSQL += fmt.Sprintf(" COMMENT '%s'", idx.Comment)
			}

			if idx.Option != "" {
				createIndexSQL += " " + idx.Option
			}
			sqlStr = mg.PrintSql(createIndexSQL, values...)
		}

		return nil
	})
	return sqlStr, err
}

// gorm源码
func buildConstraint(constraint *schema.Constraint) (sql string, results []interface{}) {
	sql = "CONSTRAINT ? FOREIGN KEY ? REFERENCES ??"
	if constraint.OnDelete != "" {
		sql += " ON DELETE " + constraint.OnDelete
	}

	if constraint.OnUpdate != "" {
		sql += " ON UPDATE " + constraint.OnUpdate
	}

	var foreignKeys, references []interface{}
	for _, field := range constraint.ForeignKeys {
		foreignKeys = append(foreignKeys, clause.Column{Name: field.DBName})
	}

	for _, field := range constraint.References {
		references = append(references, clause.Column{Name: field.DBName})
	}
	results = append(results, clause.Table{Name: constraint.Name}, foreignKeys, clause.Table{Name: constraint.ReferenceSchema.Table}, references)
	return
}

func (mg *MySqlGenerator) PrintSql(sql string, values ...interface{}) string {
	tx := mg.DB
	tx.Statement.SQL = strings.Builder{}
	var sqlStr string
	if strings.Contains(sql, "@") {
		clause.NamedExpr{SQL: sql, Vars: values}.Build(tx.Statement)
	} else {
		clause.Expr{SQL: sql, Vars: values}.Build(tx.Statement)
		sqlStr = tx.Statement.SQL.String()
	}
	return sqlStr
}
