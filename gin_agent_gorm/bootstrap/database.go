package bootstrap

import (
	"fmt"
	"time"

	// GORM 的 MySQL 数据库驱动导入
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"
)

// setupDB 初始化数据库和 ORM
func setupDB() {

	console.Info("init database ...")

	switch config.GetString("cfg.database.driver") {
	case "mysql":
		setupDBMySQL()
	default:
		console.Exit("database driver not supported")
	}

}

func setupDBMySQL() {

	configs := config.Get("cfg.database.mysql")

	dbConfigs := make(map[string]*database.DBClientConfig)

	for group := range configs.(map[string]interface{}) {
		cfgPrefix := "cfg.database.mysql." + group + "."
		username := config.GetString(cfgPrefix + "username")
		password := config.GetString(cfgPrefix + "password")
		host := config.GetString(cfgPrefix + "host")
		port := config.GetString(cfgPrefix + "port")
		db := config.GetString(cfgPrefix + "database")
		charset := config.GetString(cfgPrefix + "charset")
		collation := config.GetString(cfgPrefix+"collation", "utf8mb4_0900_ai_ci")

		// 构建 dsn 信息。DSN 全称为 Data Source Name，表示【数据源信息】
		// user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&collation=%s",
			username, password, host, port, db, charset, collation)

		var dbConfig gorm.Dialector
		dbConfig = mysql.New(mysql.Config{
			DSN: dsn,
		})

		var cfg database.DBClientConfig
		cfg.DBConfig = dbConfig
		cfg.LG = logger.NewGormLogger()
		cfg.MaxOpenConns = config.GetInt(cfgPrefix + "max_open_connections")
		cfg.MaxIdleConns = config.GetInt(cfgPrefix + "max_idle_connections")
		cfg.ConnMaxLifetime = time.Duration(config.GetInt(cfgPrefix+"max_life_seconds")) * time.Second

		dbConfigs[group] = &cfg
	}

	database.ConnectMySQL(dbConfigs)

	fixDatabaseCollation(dbConfigs)
}

func fixDatabaseCollation(dbConfigs map[string]*database.DBClientConfig) {
	console.Info("fixing database collation...")

	for group := range dbConfigs {
		db := database.Instance(group).DB

		dbname := config.GetString("cfg.database.mysql." + group + ".database")
		charset := config.GetString("cfg.database.mysql." + group + ".charset")
		collation := config.GetString("cfg.database.mysql."+group+".collation", "utf8mb4_0900_ai_ci")

		console.Info("fixing collation for database: " + dbname)

		err := db.Exec(fmt.Sprintf("ALTER DATABASE `%s` CHARACTER SET %s COLLATE %s", dbname, charset, collation)).Error
		if err != nil {
			console.Danger("failed to alter database collation: " + err.Error())
			continue
		}

		var tables []string
		err = db.Table("information_schema.tables").
			Where("table_schema = ?", dbname).
			Pluck("table_name", &tables).Error
		if err != nil {
			console.Danger("failed to get tables: " + err.Error())
			continue
		}

		console.Info(fmt.Sprintf("found %d tables", len(tables)))

		for _, table := range tables {
			console.Info("fixing collation for table: " + table)
			err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET %s COLLATE %s", table, charset, collation)).Error
			if err != nil {
				console.Danger("failed to alter table " + table + ": " + err.Error())
			}
		}

		console.Info("collation fixed for database: " + dbname)
	}

	console.Info("database collation fix completed")
}
