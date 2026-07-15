package agent

import (
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"regexp"
)

type Config struct {
	RPCServerAddress string               `yaml:"RPCServerAddress"`
	HTTPAddress      string               `yaml:"HTTPAddress"`
	LogLevel         string               `yaml:"logLevel"`
	CollectAndReport bool                 `yaml:"collectAndReport"`
	Log              bool                 `yaml:"log"`
	LogStrategies    []*model.LogStrategy `yaml:"logStrategies"`
	Task             TaskSection          `yaml:"task"`
	Region           string               `yaml:"region"`
}

type TaskSection struct {
	MetaDir  string `yaml:"metaDir"`
	Interval int    `yaml:"interval"`
}

func SetLogRegs(input []*model.LogStrategy) []*model.LogStrategy {
	res := []*model.LogStrategy{}
	for _, st := range input {
		st := st
		st.TagRegs = make(map[string]*regexp.Regexp)
		// 处理主正则
		if len(st.Pattern) != 0 {
			reg, err := regexp.Compile(st.Pattern)
			if err != nil {
				fmt.Printf("compile pattern regexp failed:[name:%v][pat:%v][err:%v]\n",
					st.MetricName,
					st.Pattern,
					err,
				)
				continue
			}
			st.PatternRegs = reg
		}
		// 处理标签的正则
		for tagK, tagV := range st.Tags {
			reg, err := regexp.Compile(tagV)
			if err != nil {
				fmt.Printf("compile pattern regexp failed:[name:%v][pat:%v][err:%v]\n",
					st.MetricName,
					tagV,
					err,
				)
				continue
			}
			st.TagRegs[tagK] = reg
		}
		res = append(res, st)
	}
	return res
}
