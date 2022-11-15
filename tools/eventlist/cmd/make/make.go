package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/josephspurrier/goversioninfo"
)

const program = "eventlist"
const mainPath = "./cmd/" + program
const resourceFileName = "resource.syso"
const emptyString = ""
const seperator = "#"
const unknownVersion = "0.0.0"

var buildDir = "build"
var legalCopyright = "Copyright (C) 2022, Arm Limited and Contributors. All rights reserved."

// Errors
var ErrGitTag = errors.New("git tag error")
var ErrVersion = errors.New("version error")
var ErrCommand = errors.New("command error")

func reportError(err error, msg string) error {
	return fmt.Errorf("%w: %s", err, msg)
}

type buildArguments struct {
	targetOs   string
	targetArch string
	outDir     string
}

type runner struct {
	buildArgs buildArguments
	testArgs  []string
}

func (r runner) run(command string) {
	switch {
	case command == "build":
		_ = os.Mkdir(buildDir, os.ModePerm)
		versionStr, CopyrightStr, err := createResourceInfoFile(r.buildArgs.targetArch)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		versionInfo := versionStr + seperator + CopyrightStr
		if err = r.build(r.buildArgs, versionInfo); err != nil {
			fmt.Println(err.Error())
		}
	case command == "test":
		if err := r.test(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "clean":
		r.clean()
	case command == "coverage":
		if err := r.coverage(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "coverage-report":
		if err := r.coverageReport(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "lint":
		r.lint()
	case command == "format":
		r.format()
	}
}

func (r runner) executeCommand(command string) (err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	if stdoutStr != emptyString {
		fmt.Println(stdoutStr)
	}
	if stderrStr != emptyString {
		fmt.Println(stderrStr)
	}
	return err
}

func (r runner) build(buildArgs buildArguments, versionInfo string) (err error) {
	var extn string
	if buildArgs.targetOs == emptyString {
		buildArgs.targetOs = runtime.GOOS
	}
	if buildArgs.targetArch == emptyString {
		buildArgs.targetArch = runtime.GOARCH
	}
	if buildArgs.outDir != emptyString {
		buildDir = buildArgs.outDir
	}
	if buildArgs.targetOs == "windows" {
		extn = ".exe"
	}

	cmd := "GOOS=" + buildArgs.targetOs + " GOARCH=" + buildArgs.targetArch +
		" go build -ldflags '-X \"main.versionInfo=" + versionInfo +
		"\"' -o " + buildDir + "/" + program + extn + " " + mainPath

	if err = r.executeCommand(cmd); err == nil {
		fmt.Println("build finished successfully!")
	}
	return err
}

func (r runner) test() (err error) {
	args := "./..."
	if len(r.testArgs) != 0 {
		args = strings.Join(r.testArgs[:], " ")
	}
	return r.executeCommand("go test " + args)
}

func (r runner) clean() {
	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		os.RemoveAll(buildDir)
		os.Remove(buildDir)
	}
	resourceFile := mainPath + "/" + resourceFileName
	if _, err := os.Stat(resourceFile); !os.IsNotExist(err) {
		os.Remove(resourceFile)
	}
	fmt.Println("cleaned successfully!")
}

func (r runner) coverage() (err error) {
	_ = os.Mkdir(buildDir, os.ModePerm)
	return r.executeCommand("go test ./... -coverprofile " + buildDir + "/cover.out")
}

func (r runner) coverageReport() (err error) {
	if err = r.coverage(); err != nil {
		return err
	}
	return r.executeCommand("go tool cover -html=" + buildDir + "/cover.out")
}

func (r runner) lint() {
	_ = r.executeCommand("golangci-lint run --config=./.golangci.yaml")
}

func (r runner) format() {
	_ = r.executeCommand("gofmt -s -w .")
}

func fetchVersionInfoFromGit() (version version, err error) {
	out, err := exec.Command("git", "describe", "--tags", "--match", "tools/eventlist/*").Output()
	if len(out) == 0 && err != nil {
		fmt.Println("warning: no release tag found, setting version to default \"0.0.0\"")
		return newVersion(unknownVersion)
	}
	if err != nil {
		return
	}
	tag := strings.TrimSpace(string(out))
	if tag == emptyString {
		return version, reportError(ErrGitTag, "no git release tag found")
	}
	tokens := strings.Split(tag, "/")
	if len(tokens) != 3 {
		return version, reportError(ErrGitTag, "invalid release tag")
	}
	return newVersion(tokens[2])
}

func createResourceInfoFile(arch string) (version string, copyright string, err error) {
	gitVersion, err := fetchVersionInfoFromGit()
	if err != nil {
		return
	}

	verInfo := goversioninfo.VersionInfo{}

	verInfo.FixedFileInfo.FileVersion = goversioninfo.FileVersion{
		Major: gitVersion.major,
		Minor: gitVersion.minor,
		Patch: gitVersion.patch,
		Build: gitVersion.numCommit,
	}
	verInfo.FixedFileInfo.ProductVersion = verInfo.FixedFileInfo.FileVersion
	verInfo.StringFileInfo = goversioninfo.StringFileInfo{
		FileDescription:  program,
		InternalName:     program,
		ProductName:      program,
		OriginalFilename: program + ".exe",
		FileVersion:      gitVersion.String(),
		ProductVersion:   gitVersion.String(),
		LegalCopyright:   legalCopyright,
	}
	verInfo.VarFileInfo.Translation = goversioninfo.Translation{
		LangID:    1033,
		CharsetID: 1200,
	}

	// Fill the structures with config data
	verInfo.Build()
	// Write the data to a buffer
	verInfo.Walk()

	version = verInfo.StringFileInfo.FileVersion
	copyright = verInfo.StringFileInfo.LegalCopyright

	return version, copyright,
		verInfo.WriteSyso(mainPath+"/"+resourceFileName, arch)
}

func isCommandValid(command string) (result bool) {
	for _, cmd := range []string{
		"build", "clean", "coverage", "coverage-report",
		"format", "help", "lint", "test",
	} {
		if cmd == command {
			return true
		}
	}
	fmt.Println(reportError(ErrCommand, "invalid command").Error())
	return false
}

type version struct {
	major, minor, patch int
	numCommit           int
	shaCommit           string
}

func (v version) String() string {
	if v.shaCommit == emptyString && v.numCommit == 0 {
		return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	}
	return fmt.Sprintf("%d.%d.%d-dev%d+%s", v.major, v.minor, v.patch, v.numCommit, v.shaCommit)
}

func newVersion(verStr string) (ver version, err error) {
	versionStr := strings.TrimSpace(verStr)
	tokens := strings.Split(versionStr, "-")
	numTokens := len(tokens)

	if !(numTokens == 1 || numTokens == 3) {
		return ver, reportError(ErrVersion, "invalid version string")
	}
	verParts := strings.Split(tokens[0], ".")
	if len(verParts) != 3 {
		return ver, reportError(ErrVersion, "invalid version string")
	}

	// Major
	ver.major, err = strconv.Atoi(verParts[0])
	if err != nil {
		return version{}, err
	}
	// Minor
	ver.minor, err = strconv.Atoi(verParts[1])
	if err != nil {
		return version{}, err
	}
	// Patch
	ver.patch, err = strconv.Atoi(verParts[2])
	if err != nil {
		return version{}, err
	}

	if numTokens == 3 {
		// Number of commits
		ver.numCommit, err = strconv.Atoi(tokens[1])
		if err != nil {
			return version{}, err
		}
		// SHA of commit
		ver.shaCommit = tokens[2]
	}
	return ver, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(reportError(ErrCommand, "invalid command").Error())
		os.Exit(1)
	}

	command := os.Args[1]
	if !isCommandValid(command) {
		os.Exit(1)
	}

	commFlag := flag.CommandLine
	targetOs := commFlag.String("os", runtime.GOOS, "Target Operating System")
	targetArch := commFlag.String("arch", runtime.GOARCH, "Target architecture")
	outDir := commFlag.String("outdir", "build", "Output directory")
	_ = commFlag.Parse(os.Args[2:])

	var testArgs []string
	if command == "test" {
		testArgs = commFlag.Args()
	}

	runner := runner{
		buildArgs: buildArguments{
			targetOs:   *targetOs,
			targetArch: *targetArch,
			outDir:     *outDir,
		},
		testArgs: testArgs,
	}
	runner.run(command)
}
