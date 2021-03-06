// +build linux darwin freebsd

package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

var (
	fileSuffix = "kvexpress"
)

// ReadFile reads a file in the filesystem and returns a string.
func ReadFile(filepath string) string {
	dat, err := ioutil.ReadFile(filepath)
	if err != nil {
		dat = []byte("")
	}
	return string(dat)
}

// SortFile takes a string, splits it into lines, removes all blank lines using
// BlankLineStrip() and then sorts the remaining lines.
func SortFile(file string) string {
	Log("sorting='true'", "debug")
	lines := strings.Split(file, "\n")
	lines = BlankLineStrip(lines)
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// BlankLineStrip takes a slice of strings, ranges over them and only returns
// a slice of strings where the lines weren't blank.
func BlankLineStrip(data []string) []string {
	Log(fmt.Sprintf("in: stripping_blank_lines='true'"), "debug")
	var stripped []string
	for _, str := range data {
		if str != "" {
			stripped = append(stripped, str)
		}
	}
	return stripped
}

// CheckFullPath will check the path and recursively create directories if they don't
// exist.
func CheckFullPath(file string) {
	targetDirectory := path.Dir(file)
	// If there is a file with the same name in the targetDirectory path - it will error.
	// It will not overwrite it.
	err := os.MkdirAll(targetDirectory, os.FileMode(0755))
	if err != nil {
		Log(fmt.Sprintf("function='CheckFullPath' panic='true' file='%s'", targetDirectory), "info")
		fmt.Printf("Panic: Could not create directories: '%s'\n", targetDirectory)
		StatsdPanic(targetDirectory, "create_directory")
	}
}

// WriteFile writes a string to a filepath. It also chowns the file to the owner and group
// of the user running the program if it's not set as a different user.
func WriteFile(data string, filepath string, perms int, owner string) {
	// If a directory doesn't exist then that's a bad thing.
	// Caused some problems with Consul and file descriptors after a long weekend erroring.
	CheckFullPath(filepath)
	// Write the file to the tmpFilepath.
	tmpFilepath := fmt.Sprintf("%s.%s", filepath, fileSuffix)
	err := ioutil.WriteFile(tmpFilepath, []byte(data), os.FileMode(perms))
	if err != nil {
		Log(fmt.Sprintf("function='WriteFile' panic='true' file='%s'", filepath), "info")
		fmt.Printf("Panic: Could not write file: '%s'\n", filepath)
		StatsdPanic(filepath, "write_file")
	}
	// Chown the file.
	fileChown, oid, gid := ChownFile(tmpFilepath, owner)
	// Rename the file so it's not truncated for 1 microsecond
	// which is actually important at high velocities.
	err = os.Rename(tmpFilepath, filepath)
	if err != nil {
		Log(fmt.Sprintf("function='Rename' panic='true' file='%s'", filepath), "info")
		fmt.Printf("Panic: Could not rename file: '%s'\n", filepath)
		StatsdPanic(filepath, "rename_file")
	}
	Log(fmt.Sprintf("file_wrote='true' location='%s' permissions='%s'", filepath, strconv.FormatInt(int64(perms), 8)), "debug")
	Log(fmt.Sprintf("file_chown='%t' location='%s' owner='%d' group='%d'", fileChown, filepath, oid, gid), "debug")
}

// ChownFile does what it sounds like.
func ChownFile(filepath string, owner string) (bool, int, int) {
	var fileChown = false
	oid := GetOwnerID(owner)
	gid := GetGroupID(owner)
	err := os.Chown(filepath, oid, gid)
	if err != nil {
		fileChown = false
		fmt.Printf("Panic: Could not chown file: '%s'\n", filepath)
		StatsdPanic(filepath, "chown_file")
	} else {
		fileChown = true
	}
	return fileChown, oid, gid
}

// CheckFiletoWrite takes a filename and checksum and stops execution if
// there is a directory OR the file has the same checksum.
func CheckFiletoWrite(filename, checksum string) {
	// Try to open the file.
	file, err := os.Open(filename)
	f, err := file.Stat()
	switch {
	case err != nil:
		Log(fmt.Sprintf("there is NO file at %s", filename), "debug")
		break
	case f.IsDir():
		Log(fmt.Sprintf("Can NOT write a directory %s", filename), "info")
		os.Exit(1)
	default:
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			Log(fmt.Sprintf("CheckFiletoWrite(): Error reading file: '%s'", filename), "info")
		}
		computedChecksum := ComputeChecksum(string(data))
		if computedChecksum == checksum {
			Log(fmt.Sprintf("'%s' has the same checksum. Stopping.", filename), "info")
			os.Exit(0)
		}
	}
	// If there's no file - then great - there's nothing to check
}

// RemoveFile takes a filename and stops if it's a directory. It will log success
// or failure of removal.
func RemoveFile(filename string) {
	file, err := os.Open(filename)
	f, err := file.Stat()
	switch {
	case err != nil:
		Log(fmt.Sprintf("Could NOT stat %s", filename), "debug")
	case f.IsDir():
		Log(fmt.Sprintf("Would NOT remove a directory %s", filename), "info")
		os.Exit(1)
	default:
		err = os.Remove(filename)
		if err != nil {
			Log(fmt.Sprintf("Could NOT remove %s", filename), "info")
		} else {
			Log(fmt.Sprintf("Removed %s", filename), "info")
		}
	}
}

// RandomTmpFile is used to create a .compare or .last file for UrltoRead()
func RandomTmpFile() string {
	file, err := ioutil.TempFile(os.TempDir(), "kvexpress")
	if err != nil {
		Log("function='RandomTmpFile' panic='true'", "info")
	}
	fileName := file.Name()
	Log(fmt.Sprintf("tempfile='%s'", fileName), "debug")
	return fileName
}

// CompareFilename returns a .compare filename based on the passed file.
func CompareFilename(file string) string {
	compare := fmt.Sprintf("%s.compare", path.Base(file))
	fullPath := path.Join(path.Dir(file), compare)
	Log(fmt.Sprintf("file='compare' fullPath='%s'", fullPath), "debug")
	return fullPath
}

// LastFilename returns a .last filename based on the passed file.
func LastFilename(file string) string {
	last := fmt.Sprintf("%s.last", path.Base(file))
	fullPath := path.Join(path.Dir(file), last)
	Log(fmt.Sprintf("file='last' fullPath='%s'", fullPath), "debug")
	return fullPath
}

// CheckLastFile creates a .last file if it doesn't exist.
func CheckLastFile(file string, perms int, owner string) {
	if _, err := os.Stat(file); err != nil {
		Log(fmt.Sprintf("file='last' file='%s' does_not_exist='true'", file), "debug")
		WriteFile("This is a blank file.\n", file, perms, owner)
	}
}

// LockFilePath generates a filename for the `$filename.locked` files used
// by `kvexpress lock` and `kvexpress unlock`
func LockFilePath(file string) string {
	lockedFile := fmt.Sprintf("%s.locked", file)
	return lockedFile
}

// LockFileWrite writes a `$filename.locked` file with instructions for how to unlock.
func LockFileWrite(file string) {
	lockedFile := LockFilePath(file)
	if _, err := os.Stat(lockedFile); err != nil {
		Log(fmt.Sprintf("file='locked' file='%s' does_not_exist='true'", lockedFile), "debug")
		lockedFileText := fmt.Sprintf("To unlock '%s' and allow kvexpress to write again:\n\nsudo kvexpress unlock -f %s\n\nReason Locked: %s\n\n", FiletoLock, FiletoLock, LockReason)
		WriteFile(lockedFileText, lockedFile, FilePermissions, Owner)
	} else {
		Log(fmt.Sprintf("file='locked' file='%s' does_not_exist='false'", lockedFile), "info")
	}
}

// LockFileRemove removes a `$filename.locked` when running `kvexpress unlock`.
func LockFileRemove(file string) {
	lockedFile := LockFilePath(file)
	RemoveFile(lockedFile)
}

// CheckFullFilename makes sure that the filename begins with a slash.
func CheckFullFilename(file string) {
	if !strings.HasPrefix(file, "/") {
		fmt.Println("Please supply a complete file path.")
		os.Exit(1)
	}
}
