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
	"os"
	"strings"

	"github.com/fatih/color"
)

type EnvironmentVars map[string]string

func (ev EnvironmentVars) Export() []string {
	ans := make([]string, len(ev))
	for k, v := range ev {
		ans = append(ans, k+"="+v)
	}
	return ans
}

func (ev EnvironmentVars) Print(linePrefix string) {
	for k, v := range ev {
		fmt.Printf("%s=%s\n", color.HiGreenString("%s%s", linePrefix, k), v)
	}
}

func (ev EnvironmentVars) UpdateBy(other EnvironmentVars) {
	for k, v := range other {
		ev[k] = v
	}
}

func GetEnvironmentVars() EnvironmentVars {
	parsed := make(EnvironmentVars)
	for _, env := range os.Environ() {
		item := strings.SplitN(env, "=", 2)
		parsed[item[0]] = item[1]
	}
	return parsed
}
