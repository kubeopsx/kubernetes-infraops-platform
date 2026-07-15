package consumer

import (
	"bytes"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/logger"
	"math"
	"sort"
	"strconv"
	"time"
)

type Consumer struct {
	FilePath     string
	Stream       chan string        // 接收生产者chan
	S            *model.LogStrategy // 策略
	Mark         string             // worker 名称，方便后续排查
	Close        chan struct{}
	CounterQueue chan *AnalystPoint
	// 统计的字段
	Analyzing bool // 正在分析日志
}

type AnalystPoint struct {
	Value           float64 // 数字的正则结果，cnt 计数的时候就是NaN
	MetricsName     string
	LogFunc         string // 计算的方法， cnt/max/min
	SortLabelString string // 标签排序结果
	LabelMap        map[string]string
}

func (c *Consumer) Start() {
	go func() {
		c.worker()
	}()
}

func (c *Consumer) Stop() {
	close(c.Close)
}

func (c *Consumer) worker() {
	logger.Infof("[Consumer:%v] starting...{}", c.Mark)
	var anaCnt, anaSwp int64
	analysClose := make(chan struct{})
	go func() {
		for {
			select {
			case <-analysClose:
				return
			case <-time.After(time.Second * 10):

			}
			a := anaCnt
			logger.Infof("[Consumer:%v][analysis %d line in last 10s]", c.Mark, a-anaSwp)
			anaSwp = a
		}
	}()

	for {
		select {
		case line := <-c.Stream:
			anaCnt++
			c.Analyzing = true
			c.analysis(line)
			c.Analyzing = false

		case <-c.Close:
			analysClose <- struct{}{}
			return
		}

	}
}

func (c *Consumer) analysis(line string) {
	var (
		patternReg = c.S.PatternRegs
		value      = math.NaN()
		vString    string // 非 cnt的正则 数字分组
	)

	// 处理日志主正则
	v := patternReg.FindStringSubmatch(line)

	/*
		## 处理日志主正则
		 - patternReg.FindStringSubmatch(line) 的结果v
		 - len=0 说明 正则没匹配中，应该丢弃这行
		 - len=1 说明 正则匹配中了，但是小括号分组没匹配到
		 - len>1 说明 正则匹配中了，小括号分组也匹配到
	*/

	if len(v) == 0 {
		// 正则没匹配中，应该丢弃这行
		return
	}

	logger.Infof("[mark:%v][line:%v][reg_res:%v]", c.Mark, line, v)

	if len(v) > 1 {
		// len>1 说明 正则匹配中了，小括号分组也匹配到
		vString = v[1]
	}

	// 如果value能被解析成float，说明匹配的 正则分组 应该是 code=200
	value, _ = strconv.ParseFloat(vString, 64)

	// 处理 tag 的正则
	labelMap := map[string]string{}
	for key, regTag := range c.S.TagRegs {
		labelMap[key] = ""
		t := regTag.FindStringSubmatch(line)
		if t != nil && len(t) > 1 {
			labelMap[key] = t[1]
		}
	}

	ret := &AnalystPoint{
		Value:           value,
		MetricsName:     c.S.MetricName,
		LogFunc:         c.S.Func,
		SortLabelString: SortedTags(labelMap),
		LabelMap:        labelMap,
	}
	c.CounterQueue <- ret
}

func SortedTags(tags map[string]string) string {
	if tags == nil {
		return ""
	}

	size := len(tags)
	if size == 0 {
		return ""
	}

	r := new(bytes.Buffer)

	if size == 1 {
		for k, v := range tags {
			r.WriteString(k)
			r.WriteString("=")
			r.WriteString(v)
		}
		return r.String()
	}

	keys := make([]string, size)
	i := 0
	for k := range tags {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	for j, key := range keys {
		r.WriteString(key)
		r.WriteString("=")
		r.WriteString(tags[key])
		if j != size-1 {
			r.WriteString(",")
		}
	}
	return r.String()
}
