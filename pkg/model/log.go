package model

import (
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"regexp"
)

type LogStrategy struct {
	ID          int64                     `json:"id" yaml:"-" xorm:"'id'"`
	MetricName  string                    `json:"metric_name" yaml:"metric_name" `
	MetricHelp  string                    `json:"metric_help" yaml:"metric_help" `
	FilePath    string                    `json:"file_path" yaml:"file_path"`
	Pattern     string                    `json:"pattern" yaml:"pattern"`
	Func        string                    `json:"func" yaml:"func"`
	Creator     string                    `json:"creator"`
	Tags        map[string]string         `json:"-" yaml:"tags" xorm:"-"`     // yaml 用的
	TagJson     string                    `json:"tag_json" yaml:"-" xorm:"-"` // db 用的
	TagRegs     map[string]*regexp.Regexp `json:"-" yaml:"-" xorm:"-"`        // 标签正则
	PatternRegs *regexp.Regexp            `json:"-" yaml:"-" xorm:"-"`        // 主正则
}

func (lm *LogStrategy) TableName() string {
	return "logging"
}

func Gets(where string, args ...interface{}) ([]*LogStrategy, error) {
	var obj []*LogStrategy
	err := db.Database["stree"].Table("logging").Where(where, args...).Find(&obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (lm *LogStrategy) Add() (int64, error) {
	_, err := db.Database["stree"].InsertOne(lm)
	return lm.ID, err
}
