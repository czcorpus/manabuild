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
	"regexp"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
)

var (
	VerSrchPtrn = regexp.MustCompile(`open-([^\s]+)`)
)

type Version struct {
	Major   int
	Minor   int
	Patch   int
	Variant string
}

func (v Version) Semver() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) String() string {
	var vs string
	if v.Variant != "" {
		vs = "-" + vs
	}
	return fmt.Sprintf(
		"manatee-open-%d.%d.%d%s", v.Major, v.Minor, v.Patch, vs)
}

func (v Version) Ge(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor > other.Minor
	}
	return v.Patch > other.Patch
}

func (v Version) Eq(other Version) bool {
	return v.Major == other.Major && v.Minor == other.Minor && v.Patch == other.Patch
}

func ParseManateeVersion(v string) (Version, error) {
	v = strings.TrimSuffix(v, "-cnc")
	items := strings.Split(v, ".")
	if len(items) < 2 || len(items) > 3 {
		return Version{}, fmt.Errorf("invalid version specifier: %s", v)
	}
	var err error
	var ans Version
	ans.Major, err = strconv.Atoi(items[0])
	if err != nil {
		return ans, err
	}
	ans.Minor, err = strconv.Atoi(items[1])
	if err != nil {
		return ans, err
	}
	if len(items) == 3 {
		ans.Patch, err = strconv.Atoi(items[2])
		return ans, err
	}
	return ans, nil
}

func AutodetectManateeVersion(specPath string) (Version, error) {
	tpl := `
		import manatee
		print manatee.version()
	`
	if specPath != "" {
		tpl = fmt.Sprintf(
			`
				sys.path.insert(0, %s)
				import manatee
				print manatee.version()
			`,
			specPath,
		)
	}
	cmd := exec.Command("python3", "-c", tpl)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return ParseManateeVersion(string(out))
	}
	if err != nil {
		libPath := DefaultManateeLibPath
		if specPath != "" {
			libPath = path.Join(specPath, "libmanatee.so")
		}
		cmd := exec.Command("strings", libPath)
		out, err = cmd.CombinedOutput()
		if err == nil {
			srch := VerSrchPtrn.FindStringSubmatch(string(out))
			return ParseManateeVersion(srch[1])

		} else {
			err = fmt.Errorf("failed to run `strings %s`", libPath)

		}
	}
	return Version{}, fmt.Errorf("failed to find Manatee version: %w", err)
}

func findManatee() string {
	if fs.PathExists("/usr/lib/libmanatee.so") {
		return "/usr/lib"
	}
	if fs.PathExists("/usr/local/lib/libmanatee.so") {
		return "/usr/local/lib"
	}
	return ""
}
