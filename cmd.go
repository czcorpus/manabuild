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
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/fatih/color"
)

var (
	version   string
	buildDate string
	gitCommit string

	KnownVersions = []string{
		"2.167.8",
		"2.167.10",
		"2.208",
		"2.214.1",
		"2.223.6",
	}
)

func showVersionMismatch(found, expected Version) {
	fmt.Printf("\nERROR: Found Manatee %s, you require %s.\n", found.Semver(), expected.Semver())
	fmt.Println("\nA) If you prefer a different installed version of Manatee")
	fmt.Println("then please specify a path where a respective libmanatee.so")
	fmt.Printf("can be found (manabuild %s --manatee-lib /path/to/libmanatee.so/dir\n\n", expected.Semver())
	fmt.Println("B) If you want to use the detected installed version then run")
	fmt.Printf("this script with proper version (manabuild %s)", found.Semver())
}

func mkHeader() {

	repeatStr := func(str string, n int) string {
		b := strings.Builder{}
		for i := 0; i < n; i++ {
			b.WriteString(str)
		}
		return b.String()
	}

	verInfo := fmt.Sprintf(
		"|  manabuild %s, build date: %s, last commit: %s  |", version, buildDate, gitCommit)
	hd := fmt.Sprintf("+%s+", repeatStr("-", len(verInfo)-2))
	fmt.Println(hd)
	fmt.Println(verInfo)
	fmt.Println(hd)
}

func clearPreviousBinaries(workingDir, binaryName string) {
	binPath := path.Join(workingDir, fmt.Sprintf("%s.bin", binaryName))
	rsPath := path.Join(workingDir, binaryName)
	os.Remove(binPath)
	os.Remove(rsPath)
}

func generateBootstrapScript(
	ctx *OperationSequence,
	needsLDScript bool,
	manateeLib, workingDir, binaryName string,
) error {
	binPath := path.Join(workingDir, fmt.Sprintf("%s.bin", binaryName))
	rsPath := path.Join(workingDir, binaryName)
	if needsLDScript {
		if err := os.Rename(rsPath, binPath); err != nil {
			return err
		}
		fw, err := os.OpenFile(rsPath, os.O_WRONLY|os.O_CREATE, 0775)
		if err != nil {
			return err
		}
		fw.WriteString("#!/usr/bin/env bash\n")
		fw.WriteString("MYSELF=`which \"$0\" 2>/dev/null`\n")
		fw.WriteString(fmt.Sprintf("export LD_LIBRARY_PATH=\"%s\"\n", manateeLib))
		fw.WriteString(fmt.Sprintf("`dirname $0`/%s.bin \"${@:1}\"\n", binaryName))
		fw.Close()
		ctx.WithPausedOutput(func() {
			fmt.Print("\nGenerated run script to handle non-standard libmanatee.so location.")
			fmt.Printf(
				"\nTo install the application, copy files %s.bin and %s", binaryName, binaryName)
			fmt.Print("to a system searched path (e.g. /usr/local/bin).")
		})

	} else {
		fmt.Printf("\nTo install the application, copy file %s", binaryName)
		fmt.Print("to a system searched path (e.g. /usr/local/bin)")
	}
	return nil
}

func main() {

	flag.Usage = func() {
		fmt.Fprint(
			os.Stderr,
			"Manabuild - a tool for building Go programs with Manatee-open dependency\n",
			fmt.Sprintf("usage: %s [binary name]\n", filepath.Base(os.Args[0])),
			fmt.Sprintf("       %s [binary name] [version]", filepath.Base(os.Args[0])),
			"\n")
		flag.PrintDefaults()
	}
	conf := new(Conf)
	workingDir := flag.String("project-path", ".", "A path where a target project is located")
	err := TryConfig(*workingDir, conf)
	if err != nil && err != ErrNoConfig {
		log.Fatal(err)
	}
	shouldRunTests := flag.Bool("test", false, "Specify whether to run unit tests")
	manateeSrc := flag.String("manatee-src", "", "Location of Manatee source files")
	manateeLib := flag.String("manatee-lib", "", "Location of libmanatee.so")
	flag.Parse()
	if !conf.IsLoaded() && (flag.NArg() < 1 || flag.NArg() > 2) {
		flag.Usage()
		os.Exit(1)
	}

	mkHeader()

	if conf.SrcPath() != "" {
		color.New(color.FgHiYellow).Printf("\n \u24D8  Using %s\n", conf.SrcPath())
	}

	var shouldGenerateRunScript bool
	detectedVersion, err := AutodetectManateeVersion("")
	if err != nil {
		fmt.Printf("Failed to find manatee-open or determine its version: %s\n", err)
		os.Exit(1)
	}
	specifiedVersion := detectedVersion
	if flag.Arg(1) != "" {
		specifiedVersion, err = ParseManateeVersion(flag.Arg(1))
		if err != nil {
			fmt.Printf("Failed to parse specified version")
			os.Exit(1)
		}

	} else {
		color.New(color.FgHiYellow).Printf(
			"\n \u24D8  No explicit Manatee version specified. Found %s\n",
			detectedVersion,
		)
	}
	if flag.Arg(0) != "" {
		conf.TargetBinaryName = flag.Arg(0)
	}

	if !collections.SliceContains(KnownVersions, specifiedVersion.Semver()) {
		fmt.Printf(
			"Unsupported version: %s. Please use one of: %s\n",
			specifiedVersion, strings.Join(KnownVersions, ", "),
		)
		os.Exit(1)
	}

	timeLocation, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		fmt.Println("failed to load time location")
		os.Exit(1)
	}
	seq := NewOperationSequence(timeLocation)

	seq.RunOperation("searching for manatee-open", func(ctx *OperationSequence) {
		if *manateeSrc == "" {
			*manateeSrc, err = downloadManateeSrc(specifiedVersion)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		} else {
			ctx.WithPausedOutput(func() {
				fmt.Printf(
					"\nAssuming that provided Manatee src path matches required version %s",
					specifiedVersion.Semver(),
				)
			})
		}

		if *manateeLib == "" {
			*manateeLib = findManatee()
			if *manateeLib == "" {
				ctx.WithPausedOutput(func() {
					fmt.Println("Manatee not found in system searched paths. Please run the script with --manatee-lib argument")
				})
				os.Exit(1)
			}
			if !specifiedVersion.Eq(detectedVersion) {
				showVersionMismatch(detectedVersion, specifiedVersion)
				os.Exit(1)

			} else {
				ctx.WithPausedOutput(func() {
					fmt.Printf(
						"\nUsing system-installed %s\n",
						detectedVersion.Semver(),
					)
				})
			}

		} else {
			ctx.WithPausedOutput(func() {
				fmt.Printf(
					"\nAssuming that provided %s/libmanatee.so matches required version %s",
					*manateeLib, specifiedVersion.Semver(),
				)
			})
			shouldGenerateRunScript = true
		}
	})

	clearPreviousBinaries(*workingDir, conf.TargetBinaryName)

	seq.RunOperation("preparing manatee-open sources", func(ctx *OperationSequence) {
		err = initManateeSources(specifiedVersion, *manateeSrc)
		if err != nil {
			ctx.WithPausedOutput(func() {
				fmt.Printf("Failed to init manatee-open sources: %s", err)
			})
			os.Exit(1)
		}
	})
	seq.RunOperation("building target project", func(ctx *OperationSequence) {
		err = buildProject(
			ctx,
			specifiedVersion,
			*workingDir,
			*manateeSrc,
			*manateeLib,
			*shouldRunTests,
			conf.TargetBinaryName,
		)
		if err != nil {
			ctx.WithPausedOutput(func() {
				fmt.Printf("\U0001F4A5 Failed to build: %s\n", err)
			})
			os.Exit(1)
		}
	})

	seq.RunOperation("generating executable", func(ctx *OperationSequence) {
		generateBootstrapScript(
			ctx,
			shouldGenerateRunScript,
			*manateeLib,
			*workingDir,
			conf.TargetBinaryName,
		)
	})
}
