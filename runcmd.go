// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

type CmdWrapper struct {
	cmd        *exec.Cmd
	printIfErr bool
}

type RunCommandOption func(cmd *CmdWrapper)

func WithDir(dir string) RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.cmd.Dir = dir
	}
}

func WithEnv(env EnvironmentVars) RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.cmd.Env = env.Export()
	}
}

func WithStdout(o io.Writer) RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.cmd.Stdout = o
	}
}

func WithStderr(o io.Writer) RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.cmd.Stderr = o
	}
}

func WithPrintIfErr() RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.printIfErr = true
	}
}

func WithPrintStdout() RunCommandOption {
	return func(cmd *CmdWrapper) {
		cmd.cmd.Stdout = os.Stdout
	}
}

func RunCommand(cmd *exec.Cmd, opts ...RunCommandOption) error {
	cmdw := &CmdWrapper{
		cmd:        cmd,
		printIfErr: false,
	}
	for _, opt := range opts {
		opt(cmdw)
	}
	var err error
	var out []byte
	if cmdw.printIfErr {
		out, err = cmdw.cmd.CombinedOutput()
		if err != nil {
			color.New(color.FgHiRed).Println(string(out))
		}
		return err

	} else {
		return cmdw.cmd.Run()
	}

}

type OperationSequence struct {
	sp       *spinner.Spinner
	currIdx  int
	mtx      *sync.Mutex
	loc      *time.Location
	finished bool
}

func (seq *OperationSequence) TimeLocation() *time.Location {
	return seq.loc
}

func (seq *OperationSequence) WithPausedOutput(fn func()) {
	seq.mtx.Lock()
	defer seq.mtx.Unlock()
	if seq.sp.Active() {
		seq.sp.Stop()
		fmt.Print("")
		fn()
		seq.sp.Start()

	} else {
		fn()
	}
}

func (seq *OperationSequence) RunOperation(title string, fn func(sq *OperationSequence)) {
	if seq.finished {
		panic("operation sequence already finished")
	}
	seq.currIdx++
	color.Cyan("\n=== [%d] %s ===\n", seq.currIdx, title)

	seq.sp = spinner.New(
		spinner.CharSets[37],
		100*time.Millisecond,
		spinner.WithWriter(os.Stderr),
	)
	seq.sp.Start()
	fn(seq)
	seq.sp.Stop()
	fmt.Print("")
}

func (seq *OperationSequence) Finish() {
	seq.finished = true
}

func NewOperationSequence(loc *time.Location) *OperationSequence {
	return &OperationSequence{
		mtx:      &sync.Mutex{},
		loc:      loc,
		finished: false,
	}
}
