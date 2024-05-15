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
		"2.225.8",
	}
)

func showVersionMismatch(found, expected Version) {
	fmt.Fprintf(os.Stderr, "\nERROR: Found Manatee %s, you require %s.\n", found.Semver(), expected.Semver())
	fmt.Fprintln(os.Stderr, "\nA) If you prefer a different installed version of Manatee")
	fmt.Fprintln(os.Stderr, "then please specify a path where a respective libmanatee.so")
	fmt.Fprintf(os.Stderr, "can be found (manabuild %s --manatee-lib /path/to/libmanatee.so/dir\n\n", expected.Semver())
	fmt.Fprintln(os.Stderr, "B) If you want to use the detected installed version then run")
	fmt.Fprintf(os.Stderr, "this script with proper version (manabuild %s)", found.Semver())
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
	fmt.Fprintln(os.Stderr, hd)
	fmt.Fprintln(os.Stderr, verInfo)
	fmt.Fprintln(os.Stderr, hd)
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
			fmt.Fprint(
				os.Stderr, "\nGenerated run script to handle non-standard libmanatee.so location.")
			fmt.Fprintf(
				os.Stderr,
				"\nTo install the application, copy files %s.bin and %s", binaryName, binaryName,
			)
			fmt.Fprint(os.Stderr, " to a system searched path (e.g. /usr/local/bin).")
		})

	} else {
		fmt.Fprintf(os.Stderr, "\nTo install the application, copy file %s", binaryName)
		fmt.Fprint(os.Stderr, " to a system searched path (e.g. /usr/local/bin)")
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprint(
			os.Stderr,
			"Manabuild - a tool for building Go programs with Manatee-open dependency\n",
			fmt.Sprintf("usage: %s [binary name] (in case .manabuild.json or -no-build is enabled)\n", filepath.Base(os.Args[0])),
			fmt.Sprintf("       %s [binary name] [version]\n", filepath.Base(os.Args[0])),
			fmt.Sprintf("       %s version", filepath.Base(os.Args[0])),
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
	buildCmdDir := flag.String("cmd-dir", "", "A subdirectory of `cmd` to be used for build.")
	noBuild := flag.Bool("no-build", false, "Just check and prepare Manatee sources and define CGO variables")
	manateeSrc := flag.String("manatee-src", "", "Location of Manatee source files")
	manateeLib := flag.String("manatee-lib", "", "Location of libmanatee.so")
	flag.Parse()

	if flag.Arg(0) == "version" {
		fmt.Fprintf(
			os.Stderr,
			"Manabuild %s\nbuild date: %s\nlast commit: %s\n",
			version, buildDate, gitCommit,
		)
		os.Exit(0)
		return
	}

	if !conf.IsLoaded() && !*noBuild && (flag.NArg() < 1 || flag.NArg() > 2) {
		flag.Usage()
		os.Exit(1)
		return
	}

	mkHeader()

	if conf.SrcPath() != "" {
		color.New(color.FgHiYellow).Fprintf(os.Stderr, "\n \u24D8  Using %s\n", conf.SrcPath())
	}

	var shouldGenerateRunScript bool
	detectedVersion, err := AutodetectManateeVersion(*manateeLib, KnownVersions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find manatee-open or determine its version: %s\n", err)
		os.Exit(1)

	} else if detectedVersion.IsZero() {
		fmt.Fprintf(os.Stderr, "Autodetection has not found any suitable Manatee version. Please select one manually\n")
		os.Exit(1)
	}
	specifiedVersion := detectedVersion
	if flag.Arg(1) != "" {
		specifiedVersion, err = ParseManateeVersion(flag.Arg(1))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse specified version")
			os.Exit(1)
		}

	} else {
		color.New(color.FgHiYellow).Fprintf(
			os.Stderr,
			"\n \u24D8  No explicit Manatee version specified. Found %s\n",
			detectedVersion,
		)
	}
	if flag.Arg(0) != "" {
		conf.TargetBinaryName = flag.Arg(0)
	}

	if !collections.SliceContains(KnownVersions, specifiedVersion.Semver()) {
		fmt.Fprintf(
			os.Stderr,
			"Unsupported version: %s. Please use one of: %s\n",
			specifiedVersion, strings.Join(KnownVersions, ", "),
		)
		os.Exit(1)
	}

	timeLocation, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load time location")
		os.Exit(1)
	}
	seq := NewOperationSequence(timeLocation)

	seq.RunOperation("searching for manatee-open", func(ctx *OperationSequence) {
		if *manateeSrc == "" {
			*manateeSrc, err = downloadManateeSrc(specifiedVersion)
			if err != nil {
				ctx.Fail(func() {
					fmt.Fprintln(os.Stderr, err)
				})
			}

		} else {
			ctx.WithPausedOutput(func() {
				fmt.Fprintf(
					os.Stderr,
					"\nAssuming that provided Manatee src path matches required version %s",
					specifiedVersion.Semver(),
				)
			})
		}

		if *manateeLib == "" {
			*manateeLib = findManatee(specifiedVersion)
			if *manateeLib == "" {
				ctx.Fail(func() {
					fmt.Fprintln(
						os.Stderr,
						"Manatee not found in system searched paths. Please run the script with --manatee-lib argument")
				})
			}
			if !specifiedVersion.Eq(detectedVersion) {

				ctx.Fail(func() {
					showVersionMismatch(detectedVersion, specifiedVersion)
				})

			} else {
				ctx.WithPausedOutput(func() {
					fmt.Fprintf(
						os.Stderr,
						"\nUsing system-installed %s\n",
						detectedVersion.Semver(),
					)
				})
			}

		} else {
			ctx.WithPausedOutput(func() {
				fmt.Fprintf(
					os.Stderr,
					"\nAssuming that provided %s/libmanatee.so matches required version %s",
					*manateeLib, specifiedVersion.Semver(),
				)
			})
		}

		if !strings.HasPrefix(*manateeLib, "/usr/local/lib") { // TODO maybe we should test more paths known by LD
			shouldGenerateRunScript = true
		}
	})

	clearPreviousBinaries(*workingDir, conf.TargetBinaryName)

	seq.RunOperation("preparing manatee-open sources", func(ctx *OperationSequence) {
		err = initManateeSources(specifiedVersion, *manateeSrc)
		if err != nil {
			ctx.Fail(func() {
				fmt.Fprintf(os.Stderr, "Failed to init manatee-open sources: %s", err)
			})
		}
	})

	msg := "building target project"
	if *noBuild {
		msg = "exporting CGO variables"
	}
	seq.RunOperation(msg, func(ctx *OperationSequence) {
		err = buildProject(
			ctx,
			specifiedVersion,
			*workingDir,
			*manateeSrc,
			*manateeLib,
			*shouldRunTests,
			conf.TargetBinaryName,
			*buildCmdDir,
			*noBuild,
		)
		if err != nil {
			ctx.Fail(func() {
				fmt.Fprintf(os.Stderr, "\U0001F4A5 Failed to build: %s\n", err)
			})
		}
	})

	if !*noBuild {
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
}
