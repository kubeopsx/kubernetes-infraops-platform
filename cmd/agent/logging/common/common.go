package common

import "os"

const (
	LogQueueSize     = 1024000
	CounterQueueSize = 1024000
	LogConsumerNum   = 5
	LogFuncCnt       = "cnt"
	LogFuncSum       = "sum"
	LogFuncMax       = "max"
	LogFuncMin       = "min"
	LogFuncAvg       = "avg"
)

func GetHostName() string {
	name, _ := os.Hostname()
	return name
}
