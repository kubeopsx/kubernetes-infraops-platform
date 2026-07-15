package db

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/server"
	"time"
	"xorm.io/xorm"
	xlog "xorm.io/xorm/log"
)

var Database = map[string]*xorm.Engine{}

func InitialDatabase(cfg []*server.MysqlConfig) error {
	for _, data := range cfg {
		db, err := xorm.NewEngine("mysql", data.Addr)
		if err != nil {
			fmt.Printf("initial database err:%v", err)
		}
		db.SetConnMaxLifetime(time.Hour)
		db.SetMaxOpenConns(data.Max)
		db.SetMaxIdleConns(data.Idle)
		db.ShowSQL(data.Debug)
		db.Logger().SetLevel(xlog.LOG_INFO)
		Database[data.Name] = db
	}
	return nil
}
