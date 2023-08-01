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
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/czcorpus/cnc-gokit/fs"
)

func downloadFile(url, target string) error {
	outf, err := os.Create(target)
	if err != nil {
		return err
	}
	defer outf.Close()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("failed to download %s with status: %d", url, resp.StatusCode)
	}
	_, err = io.Copy(outf, resp.Body)
	return err
}

func unpackArchive(path string) error {
	cmd := exec.Command("tar", "xzf", path, "-C", "/tmp")
	err := cmd.Run()
	if err != nil {
		os.Remove(path)
		fmt.Printf("removing archive %s due to an error\n", path)
		return fmt.Errorf("failed to unpack file %s: %w", path, err)
	}
	return nil
}

func downloadManateeSrc(ver Version) (string, error) {
	errTpl := "Failed to download and extract manatee-open: %w. Please do this manually and run the script with --manatee-src"
	outDir := fmt.Sprintf("/tmp/manatee-open-%s", ver.Semver())
	var err error
	isDir, err := fs.IsDir(outDir)
	if err != nil {
		return "", fmt.Errorf("failed to explore directory %s: %w", outDir, err)
	}
	if isDir {
		fmt.Printf("found existing manatee directory in %s\n", outDir)
		return outDir, nil
	}
	outFile := fmt.Sprintf("/tmp/manatee-open-%s.tar.gz", ver.Semver())
	fmt.Printf("\nLooking for %s\n", path.Base(outFile))
	if !fs.PathExists(outFile) {
		url := fmt.Sprintf(
			"https://corpora.fi.muni.cz/noske/src/manatee-open/manatee-open-%s.tar.gz",
			ver.Semver())
		if err = downloadFile(url, outFile); err != nil {
			url = fmt.Sprintf(
				"https://corpora.fi.muni.cz/noske/src/manatee-open/archive/manatee-open-%s.tar.gz",
				ver.Semver())
			if err = downloadFile(url, outFile); err != nil {
				url = fmt.Sprintf(
					"http://corpora.fi.muni.cz/noske/current/src/manatee-open-%s.tar.gz",
					ver.Semver())
				if err = downloadFile(url, outFile); err != nil {
					return "", fmt.Errorf(errTpl, err)
				}
			}
		}
	}
	err = unpackArchive(outFile)
	if err != nil {
		return "", fmt.Errorf(errTpl, err)
	}
	return outDir, nil
}
