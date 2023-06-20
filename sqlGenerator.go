package gSqlHelper

import "gorm.io/gorm"

type GeneratorType int

const (
	MysqlGenerator GeneratorType = 1 << iota
)

func NewSqlGenerator(t GeneratorType, db *gorm.DB) ISqlGenerator {
	switch t {
	case MysqlGenerator:
		return NewMyGenerator(db)
	default:
		return NewMyGenerator(db)
	}
}
