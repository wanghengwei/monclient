package cmdutil

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
)

// Command todo
type Command struct {
	cmd  string
	args []string
	// true表示去掉头尾的空格。默认true
	TrimSpace bool
	// 表示总共要切分的列数，如果是-1则无上限。默认-1
	SplitNumber int
	// 是否忽略非0的返回值。默认false
	IgnoreExitCode bool
}

// NewCommand todo
func NewCommand(c string, args ...string) *Command {
	return &Command{
		cmd:            c,
		args:           args,
		TrimSpace:      true,
		SplitNumber:    -1,
		IgnoreExitCode: false,
	}
}

// Run todo
func (t *Command) Run() ([]*CommandResultLine, error) {
	c := exec.Command(t.cmd, t.args...)
	buffer := bytes.Buffer{}
	c.Stdout = &buffer
	c.Env = append(os.Environ(), "COLUMNS=1000")
	err := c.Run()
	if err != nil && !t.IgnoreExitCode {
		return nil, err
	}

	// log.Printf("%s", buffer.String())

	results := []*CommandResultLine{}
	scanner := bufio.NewScanner(&buffer)
	for scanner.Scan() {
		txt := scanner.Text()
		// log.Println(txt)
		rl := newCommandResultLine(txt, t.TrimSpace, t.SplitNumber)
		// log.Printf("%s new DONE\n", txt)
		results = append(results, rl)
		// log.Printf("%s append DONE\n", txt)
	}

	// log.Println("END")

	return results, nil
}

// RunCommand run a command and get result of stdout
// 只是方便使用的。
func RunCommand(cmd string, args ...string) ([]*CommandResultLine, error) {
	c := NewCommand(cmd, args...)
	return c.Run()
}
