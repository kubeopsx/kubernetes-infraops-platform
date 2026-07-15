package task

import (
	"bytes"
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/file"
	"github.com/toolkits/pkg/logger"
	"github.com/toolkits/pkg/sys"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

type Task struct {
	sync.Mutex

	Id      int64
	JobId   int64
	Clock   int64
	Action  string
	Status  string
	MetaDir string

	alive  bool
	Cmd    *exec.Cmd
	Stdout bytes.Buffer
	Stderr bytes.Buffer

	Script  string
	Args    string
	Account string
}

type LocalTask struct {
	M       map[int64]*Task
	MetaDir string
}

var Locals *LocalTask

func NewLocals(metaDir string) {
	Locals = &LocalTask{
		M:       make(map[int64]*Task),
		MetaDir: metaDir,
	}
}

func (lt *LocalTask) ReportTasks() []common.ReportTask {
	ret := make([]common.ReportTask, 0, len(lt.M))
	for id, t := range lt.M {
		rt := common.ReportTask{
			Id:    id,
			Clock: t.Clock,
		}
		rt.Status = t.GetStatus()
		if rt.Status == "running" || rt.Status == "killing" {
			continue
		}
		rt.Stdout = t.GetStdout()
		rt.Stderr = t.GetStderr()

		stdoutLen := len(rt.Stdout)
		stderrLen := len(rt.Stderr)

		if stdoutLen > 65535 {
			start := stdoutLen - 65535
			rt.Stdout = rt.Stdout[start:]
		}

		if stderrLen > 65535 {
			start := stderrLen - 65535
			rt.Stderr = rt.Stderr[start:]
		}
		ret = append(ret, rt)
	}
	return ret
}

func (lt *LocalTask) GetTask(id int64) (*Task, bool) {
	t, found := lt.M[id]
	return t, found
}

func (lt *LocalTask) SetTask(t *Task) {
	lt.M[t.Id] = t
}

func (lt *LocalTask) AssignTask(at *model.TaskMeta) {
	local, found := lt.GetTask(at.ID)
	if found {
		if local.Clock == at.Clock && local.Action == at.Action {
			return
		}

		local.Clock = at.Clock
		local.Action = at.Action
	} else {
		if at.Action == "kill" {
			return
		}
		local = &Task{
			Mutex:   sync.Mutex{},
			Id:      at.ID,
			JobId:   at.ID,
			Clock:   at.Clock,
			Action:  at.Action,
			MetaDir: lt.MetaDir,
			Script:  at.Script,
			Args:    at.Args,
			Account: at.Account,
		}

		lt.SetTask(local)
	}
	if local.DoneBefore() {
		local.LoadResult()
		return
	}

	if local.Action == "kill" {
		local.SetStatus("killing")
		local.Kill()
	} else if local.Action == "start" {
		local.SetStatus("running")
		local.Start()
	} else {
		logger.Warningf("unknown action: %s of task %d", at.Action, at.ID)
	}

}

func (lt *LocalTask) Clean(assigned map[int64]struct{}) {
	del := make(map[int64]struct{})
	for id := range lt.M {
		if _, found := assigned[id]; !found {
			del[id] = struct{}{}
		}
	}

	for id := range del {
		if lt.M[id].GetStatus() == "running" {
			continue
		}
		lt.M[id].ResetBuff()
		cmd := lt.M[id].Cmd
		delete(lt.M, id)
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Release()
		}
	}

}

func (t *Task) SetAlive(pa bool) {
	t.Lock()
	t.alive = pa
	t.Unlock()
}

func (t *Task) GetStdout() string {
	t.Lock()
	out := t.Stdout.String()
	t.Unlock()
	return out
}

func (t *Task) GetStderr() string {
	t.Lock()
	out := t.Stderr.String()
	t.Unlock()
	return out
}

// 设置状态
func (t *Task) SetStatus(status string) {
	t.Lock()
	t.Status = status
	t.Unlock()
}

// 获取状态
func (t *Task) GetStatus() string {
	t.Lock()
	s := t.Status
	t.Unlock()
	return s
}

// 判断是否还在运行中
func (t *Task) GetAlive() bool {
	t.Lock()
	pa := t.alive
	t.Unlock()
	return pa
}

func (t *Task) ResetBuff() {
	t.Lock()
	t.Stdout.Reset()
	t.Stderr.Reset()
	t.Unlock()
}

func (t *Task) DoneBefore() bool {
	doneFlag := path.Join(t.MetaDir, fmt.Sprint(t.Id), fmt.Sprintf("%d.done", t.Clock))
	return file.IsExist(doneFlag)
}

func (t *Task) LoadResult() {
	metadir := t.MetaDir

	doneFlag := path.Join(metadir, fmt.Sprint(t.Id), fmt.Sprintf("%d.done", t.Clock))
	stdoutFile := path.Join(metadir, fmt.Sprint(t.Id), "stdout")
	stderrFile := path.Join(metadir, fmt.Sprint(t.Id), "stderr")

	var err error

	t.Status, err = file.ReadStringTrim(doneFlag)
	if err != nil {
		log.Printf("[E] read file %s fail %v", doneFlag, err)
	}
	stdout, err := file.ReadString(stdoutFile)
	if err != nil {
		log.Printf("[E] read file %s fail %v", stdoutFile, err)
	}
	stderr, err := file.ReadString(stderrFile)
	if err != nil {
		log.Printf("[E] read file %s fail %v", stderrFile, err)
	}

	t.Stdout = *bytes.NewBufferString(stdout)
	t.Stderr = *bytes.NewBufferString(stderr)
}

func (t *Task) meta() (script string, args string, account string) {
	return
}

func (t *Task) prepare() error {
	IdDir := path.Join(t.MetaDir, fmt.Sprint(t.Id))
	err := file.EnsureDir(IdDir)
	if err != nil {
		return err
	}
	writeFlag := path.Join(IdDir, ".write")
	if file.IsExist(writeFlag) {
		argsFile := path.Join(IdDir, "args")
		args, err := file.ReadStringTrim(argsFile)
		if err != nil {
			return err
		}
		accountFile := path.Join(IdDir, "account")
		account, err := file.ReadStringTrim(accountFile)
		if err != nil {
			return err
		}

		t.Args = args
		t.Account = account
	} else {
		script, args, account := t.Script, t.Args, t.Account
		scriptFile := path.Join(IdDir, "script")
		_, err := file.WriteString(scriptFile, script)
		if err != nil {
			return err
		}
		out, err := sys.CmdOutTrim("chmod", "+x", scriptFile)
		if err != nil {
			log.Printf("[E] chmod +x %s fail %v, output: %s", scriptFile, err, out)
			return err
		}
		argsFile := path.Join(IdDir, "args")
		_, err = file.WriteString(argsFile, args)
		if err != nil {
			return err
		}

		accountFile := path.Join(IdDir, "account")
		_, err = file.WriteString(accountFile, account)
		if err != nil {
			return err
		}

		_, err = file.WriteString(writeFlag, "")
		if err != nil {
			return err
		}

		t.Args = args
		t.Account = account
	}

	return nil
}

// 启动任务
func (t *Task) Start() {
	if t.GetAlive() {
		return
	}
	err := t.prepare()
	if err != nil {
		return
	}
	args := t.Args
	if args != "" {
		args = strings.Replace(args, ",", "' '", -1)
		args = "'" + args + "'"
	}
	nowPath, _ := os.Getwd()
	scriptFile := path.Join(nowPath, t.MetaDir, fmt.Sprint(t.Id), "script")
	sh := fmt.Sprintf("%s %s", scriptFile, args)
	logger.Infof("[scriptFile:%+v][shCmd:%+v]", scriptFile, sh)
	var cmd *exec.Cmd
	if t.Account == "root" {
		cmd = exec.Command("sh", "-c", sh)
		cmd.Dir = "/root"
	} else {
		cmd = exec.Command("su", "-c", sh, "-", t.Account)
	}

	cmd.Stdout = &t.Stdout
	cmd.Stderr = &t.Stderr
	t.Cmd = cmd
	err = cmd.Start()
	if err != nil {
		return
	}
	go runProcess(t)
}

func runProcess(t *Task) {
	t.SetAlive(true)
	defer t.SetAlive(false)

	err := t.Cmd.Wait()
	if err != nil {
		if strings.Contains(err.Error(), "") {
			t.SetStatus("killed")
			logger.Debugf("task:%d", t.Id)
		} else {
			t.SetStatus("failed")
			logger.Debugf("task:%d err:%+v", t.Id, err)
		}
	} else {
		t.SetStatus("success")
		logger.Debugf("task:%d", t.Id)
	}

	persistResult(t)
}

func persistResult(t *Task) {
	metadir := t.MetaDir

	stdout := path.Join(metadir, fmt.Sprint(t.Id), "stdout")
	stderr := path.Join(metadir, fmt.Sprint(t.Id), "stderr")
	doneFlag := path.Join(metadir, fmt.Sprint(t.Id), fmt.Sprintf("%d.done", t.Clock))

	file.WriteString(stdout, t.GetStdout())
	file.WriteString(stderr, t.GetStderr())
	file.WriteString(doneFlag, t.GetStatus())
}

func (t *Task) Kill() {
	go killProcess(t)
}

// 杀进程
func killProcess(t *Task) {
	t.SetAlive(true)
	defer t.SetAlive(false)

	logger.Debugf("begin kill process of task[%d]", t.Id)

	err := KillProcessByTaskID(t.Id, t.MetaDir)
	if err != nil {
		t.SetStatus("killfailed")
		logger.Debugf("kill process of task[%d] fail: %v", t.Id, err)
	} else {
		t.SetStatus("killed")
		logger.Debugf("process of task[%d] killed", t.Id)
	}

	persistResult(t)
}

func KillProcessByTaskID(id int64, metadir string) error {
	dir := strings.TrimRight(metadir, "/")
	arr := strings.Split(dir, "/")
	lst := arr[len(arr)-1]
	return sys.KillProcessByCmdline(fmt.Sprintf("%s/%d/script", lst, id))
}
