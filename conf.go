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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

const (
	confFileName = ".manabuild.json"
)

var (
	ErrNoConfig = errors.New("config not found")
)

// Conf represents a .manabuild.json configuration file
// providing a way how to configure a building process.
type Conf struct {
	isLoaded         bool
	srcPath          string
	TargetBinaryName string `json:"targetBinaryName"`
}

func (conf *Conf) IsLoaded() bool {
	return conf.isLoaded
}

func (conf *Conf) SrcPath() string {
	return conf.srcPath
}

func TryConfig(workingDir string, conf *Conf) error {
	path := filepath.Join(workingDir, confFileName)
	if !fs.PathExists(path) {
		return ErrNoConfig
	}
	conf.srcPath = path
	rawData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	err = json.Unmarshal(rawData, conf)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	conf.isLoaded = true
	return nil
}
