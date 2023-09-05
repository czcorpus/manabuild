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
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/fatih/color"
)

const (
	DefaultManateeLibPath = "/usr/local/lib/libmanatee.so"
)

var (
	v2_208 = initV2_208()
)

func getCommitInfo(workingDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = workingDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to obtain git commit info: %w", err)
	}
	return strings.TrimSpace(string(out)), err
}

func getVersionInfo(workingDir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags")
	cmd.Dir = workingDir
	out, err := cmd.CombinedOutput()
	strOut := strings.TrimSpace(string(out))
	if err != nil {
		if strings.Contains(strOut, "No names found") {
			err = nil
			strOut = "v0.0.0"

		} else {
			err = fmt.Errorf("failed get version info: %w", err)
		}
	}
	return strOut, err
}

func getCurrentDatetime(loc *time.Location) string {
	return time.Now().In(loc).Format(time.RFC3339)
}

func initV2_208() Version {
	v2_208, err := ParseManateeVersion("2.208")
	if err != nil {
		panic(err)
	}
	return v2_208
}

func initManateeSources(version Version, manateeSrc string) error {
	isFile, err := fs.IsFile(path.Join(manateeSrc, "config.hh"))
	if err != nil {
		return err
	}

	env := GetEnvironmentVars()
	if isFile {
		// TODO probably not needed: env["PYTHON"] = "/usr/bin/python3"
		cmd := exec.Command("make", "clean")
		cmd.Dir = manateeSrc
		cmd.Env = env.Export()
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	cmd := exec.Command("./configure", "--with-pcre", "--disable-pthread")
	cmd.Env = env.Export()
	cmd.Dir = manateeSrc
	err = cmd.Run()
	if err != nil {
		return err
	}
	if version.Ge(v2_208) {
		cmd := exec.Command("make")
		cmd.Dir = path.Join(manateeSrc, "hat-trie")
		err := cmd.Run()
		if err != nil {
			return err
		}
		cmd = exec.Command("make")
		cmd.Dir = path.Join(manateeSrc, "fsa3")
		err = cmd.Run()
		return err
	}
	return nil
}

func buildProject(
	ctx *OperationSequence,
	version Version,
	workingDir,
	manateeSrc,
	manateeLib string,
	test bool,
	binaryName string,
) error {

	ver, err := getVersionInfo(workingDir)
	if err != nil {
		return err
	}
	commit, err := getCommitInfo(workingDir)
	if err != nil {
		return err
	}

	dt := getCurrentDatetime(ctx.TimeLocation())
	ldFlags := fmt.Sprintf(
		`-w -s -X main.version='%s' -X main.buildDate='%s' -X main.gitCommit='%s'`,
		ver, dt, commit,
	)
	subdirs := []string{fmt.Sprintf("-I%s", manateeSrc)}
	buildEnv := make(EnvironmentVars)
	if version.Ge(v2_208) {
		for _, dir := range []string{"finlib", "fsa3", "hat-trie"} {
			subdirs = append(subdirs, "-I"+path.Join(manateeSrc, dir))
		}
		buildEnv["CGO_CXXFLAGS"] = fmt.Sprintf(
			`-std=c++14 -I%s/corp -I%s/concord -I%s/query`, manateeSrc, manateeSrc, manateeSrc)
		buildEnv["CGO_CPPFLAGS"] = strings.Join(subdirs, " ")
		buildEnv["CGO_LDFLAGS"] = fmt.Sprintf(
			`-lmanatee -L%s -lhat-trie -L%s -lfsa3 -L%s`,
			manateeLib, manateeLib, path.Join(manateeSrc, "fsa3/.libs"))

	} else {
		buildEnv["CGO_CXXFLAGS"] = fmt.Sprintf(
			`-std=c++14 -I%s/corp -I%s/concord -I%s/query`, manateeSrc, manateeSrc, manateeSrc)
		buildEnv["CGO_CPPFLAGS"] = strings.Join(subdirs, " ")
		buildEnv["CGO_LDFLAGS"] = fmt.Sprintf(`-lmanatee -L%s`, manateeLib)
	}
	ctx.WithPausedOutput(func() {
		fmt.Println("\napplied env. variables:")
		color.Set(color.FgGreen)
		buildEnv.Print("\t")
		color.Unset()
	})
	currEnv := GetEnvironmentVars()
	currEnv.UpdateBy(buildEnv)

	var cmd *exec.Cmd
	if test {
		fmt.Println("Running TESTS:")
		cmd = exec.Command("bash", "-c", "go test ./...")
		err := RunCommand(cmd, WithDir(workingDir), WithEnv(currEnv), WithPrintStdout())
		if err != nil {
			return err
		}
	}

	fmt.Println("Running BUILD:")
	cmd = exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(`go build -o %s -ldflags "%s"`, binaryName, ldFlags),
	)
	return RunCommand(cmd, WithDir(workingDir), WithEnv(currEnv), WithPrintIfErr())
}
