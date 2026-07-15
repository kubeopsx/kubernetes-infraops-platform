package common

type ReportTask struct {
	Id     int64
	Clock  int64
	Status string
	Stdout string
	Stderr string
}
