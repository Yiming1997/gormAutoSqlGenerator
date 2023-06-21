#  gormAutoSqlGenerator  
gormAutoSqlGenerator  用在使用gorm的项目中，可以根据代码中定义的结构体实体和数据库的差异来差量化生成迁移sql。

# Why
gorm官方库目前不支持预生成迁移sql,这对于code first开发思想的团队是一件非常苦恼的事情，所以我决定使用gorm库的部分源码来把预生成sql的功能开发出来。  
 
 # Usage
 
```
import (  
 "fmt"  
  gsg "github.com/Yiming1997/gormAutoSqlGenerator" 
  "gorm.io/driver/mysql"  
  "gorm.io/gorm" 
)


mysqlConfig := mysql.Config{  
   DSN:                       "your db conn string ", // DSN data source name  
  DefaultStringSize:         191, // string 类型字段的默认长度  
  SkipInitializeWithVersion: false, // 根据版本自动配置  
  
}  
db, err := gorm.Open(mysql.New(mysqlConfig))  
if err != nil {  
   panic("conn failed")  
}  
  
myGenerator := gsg.NewSqlGenerator(gsg.MysqlGenerator, db)  
sql, _ := myGenerator.GeneratorAutoMigrationSql(  
   mim.MimUser{},  
   mim.MimMessage{},  
  //...所有实体结构体  
)
```

