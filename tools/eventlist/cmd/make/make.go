package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/josephspurrier/goversioninfo"
)

var validCommands = []string{
	"build", "clean", "coverage", "coverage-report",
	"format", "help", "lint", "test",
}

const usage = `
Usage:
  go run make.go <command> [<args>]

Commands:
  build           : Build executable
  clean           : Remove build artifacts
  coverage        : Run tests with coverage info
  coverage-report : Generate html coverage report
  format          : Align indentation and format code
  help            : Print usage
  lint            : Run linter
  test            : Run all tests

Args:
  -arch           : Target architecture for e.g amd64 etc
  -os             : Target operating system for e.g windows, linux, darwin etc
`
const program = "eventlist"
const mainPath = "./cmd/" + program
const resourceFileName = "resource.syso"
const buildDir = "build"
const emptyString = ""
const seperator = "#"
const unknownVersion = "0.0.0"

var legalCopyright = "Copyright (C) 2022%s ARM Limited or its Affiliates. All rights reserved."

// Errors
var ErrGitTag = errors.New("git tag error")
var ErrVersion = errors.New("version error")
var ErrCommand = errors.New("command error")

func reportError(err error, msg string) error {
	return fmt.Errorf("%w: %s", err, msg)
}

func executeCommand(command string) (err error) {
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

func build(buildArgs buildArguments, versionInfo string) (err error) {
	var extn string
	if buildArgs.targetOs == emptyString {
		buildArgs.targetOs = runtime.GOOS
	}
	if buildArgs.targetArch == emptyString {
		buildArgs.targetArch = runtime.GOARCH
	}
	if buildArgs.targetOs == "windows" {
		extn = ".exe"
	}

	cmd := "GOOS=" + buildArgs.targetOs + " GOARCH=" + buildArgs.targetArch +
		" go build -ldflags '-X \"main.versionInfo=" + versionInfo +
		"\"' -o " + buildDir + "/" + program + extn + " " + mainPath
	if err = executeCommand(cmd); err == nil {
		fmt.Println("build finished successfully!")
	}
	return err
}

func test() (err error) {
	return executeCommand("go test ./...")
}

func clean() {
	buildDir := "./" + buildDir
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

func coverage() (err error) {
	_ = os.Mkdir(buildDir, os.ModePerm)
	return executeCommand("go test ./... -coverprofile " + buildDir + "/cover.out")
}

func coverageReport() (err error) {
	if err = coverage(); err != nil {
		return err
	}
	return executeCommand("go tool cover -html=" + buildDir + "/cover.out")
}

func lint() {
	_ = executeCommand("golangci-lint run --config=./.golangci.yaml")
}

func format() {
	_ = executeCommand("gofmt -s -w .")
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

func updateVersionFields(version *goversioninfo.FileVersion, gitVersion version) {
	version.Major = gitVersion.major
	version.Minor = gitVersion.minor
	version.Patch = gitVersion.patch
}

func createResourceInfoFile(jsonFile string) (version string, copyright string, err error) {
	gitVersion, err := fetchVersionInfoFromGit()
	if err != nil {
		return
	}

	vi := versionInfoJSON{jsonFile: jsonFile}
	verInfo, err := vi.read()
	if err != nil {
		return
	}
	// Update Json version fields
	updateVersionFields(&verInfo.FixedFileInfo.FileVersion, gitVersion)
	updateVersionFields(&verInfo.FixedFileInfo.ProductVersion, gitVersion)
	verInfo.StringFileInfo.FileVersion = gitVersion.String()
	verInfo.StringFileInfo.ProductVersion = gitVersion.String()

	var year string
	currYear := time.Now().Year()
	if currYear != 2022 {
		year = "-" + strconv.Itoa(currYear)[2:]
	}
	verInfo.StringFileInfo.LegalCopyright = fmt.Sprintf(legalCopyright, year)

	// Fill the structures with config data
	verInfo.Build()
	// Write the data to a buffer
	verInfo.Walk()

	if err = vi.write(verInfo); err != nil {
		return
	}

	version = verInfo.StringFileInfo.FileVersion
	copyright = verInfo.StringFileInfo.LegalCopyright

	return version, copyright,
		verInfo.WriteSyso(mainPath+"/"+resourceFileName, runtime.GOARCH)
}

func isCommandValid(command string) (result bool) {
	for _, cmd := range validCommands {
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

type versionInfoJSON struct {
	jsonFile string
}

func (vi versionInfoJSON) read() (versionInfo goversioninfo.VersionInfo, err error) {
	file, err := os.Open(vi.jsonFile)
	if err != nil {
		return versionInfo, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return versionInfo, err
	}
	versionInfo = goversioninfo.VersionInfo{}
	err = json.Unmarshal(bytes, &versionInfo)
	return versionInfo, err
}

func (vi versionInfoJSON) write(versionInfo goversioninfo.VersionInfo) (err error) {
	file, err := os.Open(vi.jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()
	// Write content in JSON file
	jsonString, err := json.MarshalIndent(versionInfo, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file.Name(), jsonString, os.ModePerm)
}

type buildArguments struct {
	targetOs   string
	targetArch string
}

func main() {
	command := os.Args[1]
	if !isCommandValid(command) {
		fmt.Print(usage)
		return
	}

	commFlag := flag.CommandLine
	targetOs := commFlag.String("os", "", "Target Operating System")
	targetArch := commFlag.String("arch", "", "Target Architecture")
	_ = commFlag.Parse(os.Args[2:])

	buildArgs := buildArguments{
		targetOs:   *targetOs,
		targetArch: *targetArch,
	}
	switch {
	case command == "build":
		_ = os.Mkdir(buildDir, os.ModePerm)
		versionStr, CopyrightStr, err := createResourceInfoFile("./versioninfo.json")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		versionInfo := versionStr + seperator + CopyrightStr
		if err = build(buildArgs, versionInfo); err != nil {
			fmt.Println(err.Error())
		}
	case command == "test":
		if err := test(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "clean":
		clean()
	case command == "coverage":
		if err := coverage(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "coverage-report":
		if err := coverageReport(); err != nil {
			fmt.Println(err.Error())
			return
		}
	case command == "lint":
		lint()
	case command == "format":
		format()
	case command == "help":
		fmt.Print(usage)
	}
}
