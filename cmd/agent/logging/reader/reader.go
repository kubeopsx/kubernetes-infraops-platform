package reader

import (
	"github.com/hpcloud/tail"
	"github.com/toolkits/pkg/logger"
	"io"
	"time"
)

type Reader struct {
	FilePath    string        // 配置的日志路径
	tailer      *tail.Tail    // tailer 对象
	Stream      chan string   // 同步日志的chan
	CurrentPath string        // 当前路径
	Close       chan struct{} // 关闭的chan
	FD          uint64        // 文件的inode，用来处理文件变更的情况
}

func NewReader(filepath string, stream chan string) (*Reader, error) {
	r := &Reader{
		FilePath: filepath,
		Stream:   stream,
		Close:    make(chan struct{}),
	}
	err := r.openFile(io.SeekEnd, filepath)
	return r, err
}

func (r *Reader) openFile(whence int, filepath string) error {
	seekinfo := &tail.SeekInfo{
		Offset: 0,
		Whence: whence,
	}
	config := tail.Config{
		Location: seekinfo,
		ReOpen:   true,
		Poll:     true,
		Follow:   true,
	}
	t, err := tail.TailFile(filepath, config)
	if err != nil {
		return err
	}
	r.tailer = t
	r.CurrentPath = filepath
	r.FD = 0
	return nil
}

func (r *Reader) Start() {
	var (
		readCnt, readSwp int64
		dropCnt, dropSwp int64
	)
	analysClose := make(chan struct{})
	go func() {
		for {
			select {
			case <-analysClose:
				return
			case <-time.After(10 * time.Second):

			}
			a := readCnt
			b := dropCnt
			logger.Infof("read [%d] line in last 10s", a-readSwp)
			logger.Infof("drop [%d] line in last 10s", b-dropSwp)
			readSwp = a
			dropSwp = b
		}
	}()

	for line := range r.tailer.Lines {
		readCnt++
		select {
		case r.Stream <- line.Text:
		default:
			dropCnt++
		}
	}
	close(analysClose)
}

func (r *Reader) Stop() {
	r.stopRead()
	close(r.Close)
}

func (r *Reader) stopRead() {
	r.tailer.Stop()
}
