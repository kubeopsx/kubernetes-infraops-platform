package model

import (
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"time"
)

type TaskMeta struct {
	ID       int64     `json:"id" xorm:"id"`
	Title    string    `json:"title"`                  // 标题
	Account  string    `json:"account"`                // 脚本执行账号
	Timeout  int64     `json:"timeout"`                // 执行超时
	Script   string    `json:"script"`                 // 执行的脚本
	Args     string    `json:"args"`                   // 执行的脚本的参数
	Creator  string    `json:"creator"`                // 创建者
	Created  time.Time `json:"created" xorm:"created"` // 创建时间
	Done     int64     `json:"done" xorm:"done"`       // 任务结束与否的标志位=0未结束，=1结束
	Clock    int64     `json:"clock" xorm:"-"`
	Action   string    `json:"action"`
	Hosts    []string  `json:"-" xorm:"-"`
	HostsRaw string    `json:"hosts"` // 执行机器的ip列表json
}

type TaskResult struct {
	ID     int64  `json:"id" xorm:"id"`
	TaskID int64  `json:"task_id" xorm:"task_id"`
	Host   string `json:"host" xorm:"host" `
	Status string `json:"status" xorm:"status" `
	Stdout string `json:"stdout" xorm:"stdout"`
	Stderr string `json:"stderr" xorm:"stderr"`
}

type TaskReportRequest struct {
	AgentIp     string
	ReportTasks []common.ReportTask
}

type TaskReportResponse struct {
	Message     string
	AssignTasks []*TaskMeta
}

func GetTask(where string, args ...interface{}) (*TaskMeta, error) {
	var obj TaskMeta
	has, err := db.Database["stree"].Where(where, args...).Get(&obj)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return &obj, nil
}

func GetTaskMeta(where string, args ...interface{}) ([]*TaskMeta, error) {
	var obj []*TaskMeta
	err := db.Database["stree"].Table("task_meta").Where(where, args...).Find(&obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func UnDoTaskMeta() ([]*TaskMeta, error) {
	var obj []*TaskMeta
	session := db.Database["stree"].Where("done=0 ")
	err := session.Find(&obj)
	return obj, err
}

func DoTaskMetaMark(id int64) error {
	session := db.Database["stree"].NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return err
	}
	sql := fmt.Sprintf("UPDATE task_meta SET done=1 WHERE id=%d ", id)

	if _, err := session.Exec(sql); err != nil {
		session.Rollback()
		return err
	}
	return session.Commit()
}

func (tm *TaskMeta) Add() (int64, error) {
	_, err := db.Database["stree"].InsertOne(tm)
	return tm.ID, err
}

func (tr *TaskResult) Save() (error, bool) {
	var obj TaskResult
	sql := fmt.Sprintf(" task_id=%d and host='%s' ", tr.TaskID, tr.Host)
	has, err := db.Database["stree"].Where(sql).Get(&obj)
	if err != nil {
		return err, false
	}
	if has {
		return nil, false
	}
	_, err = db.Database["stree"].Insert(tr)
	return nil, true
}
