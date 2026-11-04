// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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

// Newer distributions (e.g. Ubuntu 26.04+) no longer ship PCRE1 (libpcre3-dev)
// which Manatee requires (it includes <pcre.h>, i.e. the PCRE1 API, so PCRE2
// cannot be used instead). This file provides a way to download, build and
// install PCRE1 into a separate prefix so it does not interfere with system
// libraries. (SWIG 3 is not needed as Manatee is configured with --disable-python.)

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
)

const (
	DefaultLegacyDepsPrefix = "/opt"
	legacyDepsWorkDir       = "/tmp/manabuild-legacy-deps"
)

// LegacyDeps describes location of locally built PCRE1
type LegacyDeps struct {
	PCREPrefix string
}

func (ld LegacyDeps) PCREBinDir() string {
	return filepath.Join(ld.PCREPrefix, "bin")
}

func (ld LegacyDeps) PCREIncludeDir() string {
	return filepath.Join(ld.PCREPrefix, "include")
}

func (ld LegacyDeps) PCRELibDir() string {
	return filepath.Join(ld.PCREPrefix, "lib")
}

// ConfigureArgs returns additional arguments for Manatee's `configure` script
func (ld LegacyDeps) ConfigureArgs() []string {
	return []string{
		"CPPFLAGS=-I" + ld.PCREIncludeDir(),
		fmt.Sprintf("LDFLAGS=-L%s -Wl,-rpath,%s", ld.PCRELibDir(), ld.PCRELibDir()),
	}
}

// UpdateEnv modifies env so `pcre-config` from the legacy PCRE is found first
func (ld LegacyDeps) UpdateEnv(env EnvironmentVars) {
	if env["PATH"] == "" {
		env["PATH"] = ld.PCREBinDir()

	} else {
		env["PATH"] = ld.PCREBinDir() + string(os.PathListSeparator) + env["PATH"]
	}
}

type legacyPackage struct {
	name          string
	url           string
	sha256        string
	srcDir        string // directory name inside the archive
	installDir    string
	configureArgs []string
	isInstalled   func() bool
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".manabuild-test-*")
	if err != nil {
		return err
	}
	tmp.Close()
	return os.Remove(tmp.Name())
}

func buildLegacyPackage(pkg legacyPackage) error {
	if pkg.isInstalled() {
		fmt.Fprintf(os.Stderr, "\nfound existing %s in %s\n", pkg.name, pkg.installDir)
		return nil
	}
	if err := ensureWritableDir(pkg.installDir); err != nil {
		return fmt.Errorf(
			"cannot install %s into %s (%w). Either run manabuild with sufficient privileges, "+
				"install the package manually or use -legacy-deps-prefix with a writable directory",
			pkg.name, pkg.installDir, err,
		)
	}

	workDir := filepath.Join(legacyDepsWorkDir, pkg.name)
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to clean up work directory %s: %w", workDir, err)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory %s: %w", workDir, err)
	}

	archive := filepath.Join(workDir, filepath.Base(strings.TrimSuffix(pkg.url, "/download")))
	fmt.Fprintf(os.Stderr, "\ndownloading %s\n", pkg.url)
	if err := downloadFile(pkg.url, archive); err != nil {
		return fmt.Errorf("failed to download %s: %w", pkg.name, err)
	}
	sum, err := sha256File(archive)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum of %s: %w", archive, err)
	}
	if sum != pkg.sha256 {
		os.Remove(archive)
		return fmt.Errorf(
			"checksum mismatch for %s (expected %s, got %s)", archive, pkg.sha256, sum)
	}

	if err := RunCommand(
		exec.Command("tar", "xf", archive, "-C", workDir), WithPrintIfErr()); err != nil {
		return fmt.Errorf("failed to unpack %s: %w", archive, err)
	}

	srcDir := filepath.Join(workDir, pkg.srcDir)
	env := GetEnvironmentVars()
	steps := [][]string{
		append([]string{"./configure", "--prefix=" + pkg.installDir}, pkg.configureArgs...),
		{"make", fmt.Sprintf("-j%d", runtime.NumCPU())},
		{"make", "install"},
	}
	for _, step := range steps {
		fmt.Fprintf(os.Stderr, "%s: running %s\n", pkg.name, strings.Join(step, " "))
		cmd := exec.Command(step[0], step[1:]...)
		if err := RunCommand(cmd, WithDir(srcDir), WithEnv(env), WithPrintIfErr()); err != nil {
			return fmt.Errorf("failed to build %s (`%s`): %w", pkg.name, strings.Join(step, " "), err)
		}
	}
	if !pkg.isInstalled() {
		return fmt.Errorf("%s build finished but the package was not found in %s", pkg.name, pkg.installDir)
	}
	return nil
}

// prepareLegacyDeps downloads, builds and installs PCRE 8.45 (last PCRE1 release)
// into prefix/pcre1. An already installed package is reused. Note: build tools
// (build-essential, automake, autoconf, autoconf-archive, libtool, pkg-config,
// libltdl-dev) must be installed.
func prepareLegacyDeps(prefix string) (LegacyDeps, error) {
	ans := LegacyDeps{
		PCREPrefix: filepath.Join(prefix, "pcre1"),
	}
	pcre := legacyPackage{
		name:          "pcre-8.45",
		url:           "https://sourceforge.net/projects/pcre/files/pcre/8.45/pcre-8.45.tar.bz2/download",
		sha256:        "4dae6fdcd2bb0bb6c37b5f97c33c2be954da743985369cddac3546e3218bffb8",
		srcDir:        "pcre-8.45",
		installDir:    ans.PCREPrefix,
		configureArgs: []string{"--enable-utf", "--enable-unicode-properties"},
		isInstalled: func() bool {
			return fs.PathExists(filepath.Join(ans.PCREBinDir(), "pcre-config")) &&
				fs.PathExists(filepath.Join(ans.PCREIncludeDir(), "pcre.h"))
		},
	}
	if err := buildLegacyPackage(pcre); err != nil {
		return LegacyDeps{}, err
	}
	return ans, nil
}
