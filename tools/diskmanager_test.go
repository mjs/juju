// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package tools_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	gc "launchpad.net/gocheck"

	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/tools"
	"launchpad.net/juju-core/version"
)

var _ = gc.Suite(&DiskManagerSuite{})

type DiskManagerSuite struct {
	coretesting.LoggingSuite
	dataDir string
	manager tools.ToolsManager
}

func (s *DiskManagerSuite) SetUpTest(c *gc.C) {
	s.LoggingSuite.SetUpTest(c)
	s.dataDir = c.MkDir()
	s.manager = tools.NewDiskManager(s.dataDir)
}

func (s *DiskManagerSuite) toolsDir() string {
	// TODO: Somehow hide this behind the DataManager
	return filepath.Join(s.dataDir, "tools")
}

// Copied from environs/agent/tools_test.go
func (s *DiskManagerSuite) TestUnpackToolsContents(c *gc.C) {
	files := []*coretesting.TarFile{
		coretesting.NewTarFile("bar", 0755, "bar contents"),
		coretesting.NewTarFile("foo", 0755, "foo contents"),
	}
	t1 := &tools.Tools{
		URL:    "http://foo/bar",
		Binary: version.MustParseBinary("1.2.3-foo-bar"),
	}

	err := s.manager.UnpackTools(t1, bytes.NewReader(coretesting.TarGz(files...)))
	c.Assert(err, gc.IsNil)
	assertDirNames(c, s.toolsDir(), []string{"1.2.3-foo-bar"})
	s.assertToolsContents(c, t1, files)

	// Try to unpack the same version of tools again - it should succeed,
	// leaving the original version around.
	t2 := &tools.Tools{
		URL:    "http://arble",
		Binary: version.MustParseBinary("1.2.3-foo-bar"),
	}
	files2 := []*coretesting.TarFile{
		coretesting.NewTarFile("bar", 0755, "bar2 contents"),
		coretesting.NewTarFile("x", 0755, "x contents"),
	}
	err = s.manager.UnpackTools(t2, bytes.NewReader(coretesting.TarGz(files2...)))
	c.Assert(err, gc.IsNil)
	assertDirNames(c, s.toolsDir(), []string{"1.2.3-foo-bar"})
	s.assertToolsContents(c, t1, files)
}

func (t *DiskManagerSuite) TestSharedToolsDir(c *gc.C) {
	manager := tools.NewDiskManager("/var/lib/juju")
	dir := manager.SharedToolsDir(version.MustParseBinary("1.2.3-precise-amd64"))
	c.Assert(dir, gc.Equals, "/var/lib/juju/tools/1.2.3-precise-amd64")
}

const urlFile = "downloaded-url.txt"

// assertToolsContents asserts that the directory for the tools
// has the given contents.
func (s *DiskManagerSuite) assertToolsContents(c *gc.C, t *tools.Tools, files []*coretesting.TarFile) {
	var wantNames []string
	for _, f := range files {
		wantNames = append(wantNames, f.Header.Name)
	}
	wantNames = append(wantNames, urlFile)
	dir := s.manager.(*tools.DiskManager).SharedToolsDir(t.Binary)
	assertDirNames(c, dir, wantNames)
	assertFileContents(c, dir, urlFile, t.URL, 0200)
	for _, f := range files {
		assertFileContents(c, dir, f.Header.Name, f.Contents, 0400)
	}
	gotTools, err := s.manager.ReadTools(t.Binary)
	c.Assert(err, gc.IsNil)
	c.Assert(*gotTools, gc.Equals, *t)
}

// assertFileContents asserts that the given file in the
// given directory has the given contents.
func assertFileContents(c *gc.C, dir, file, contents string, mode os.FileMode) {
	file = filepath.Join(dir, file)
	info, err := os.Stat(file)
	c.Assert(err, gc.IsNil)
	c.Assert(info.Mode()&(os.ModeType|mode), gc.Equals, mode)
	data, err := ioutil.ReadFile(file)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, contents)
}

// assertDirNames asserts that the given directory
// holds the given file or directory names.
func assertDirNames(c *gc.C, dir string, names []string) {
	f, err := os.Open(dir)
	c.Assert(err, gc.IsNil)
	defer f.Close()
	dnames, err := f.Readdirnames(0)
	c.Assert(err, gc.IsNil)
	sort.Strings(dnames)
	sort.Strings(names)
	c.Assert(dnames, gc.DeepEquals, names)
}
