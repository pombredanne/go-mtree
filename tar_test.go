package mtree

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func ExampleStreamer() {
	fh, err := os.Open("./testdata/test.tar")
	if err != nil {
		// handle error ...
	}
	str := NewTarStreamer(fh, nil)
	if err := extractTar("/tmp/dir", str); err != nil {
		// handle error ...
	}

	dh, err := str.Hierarchy()
	if err != nil {
		// handle error ...
	}

	res, err := Check("/tmp/dir/", dh, nil)
	if err != nil {
		// handle error ...
	}
	if len(res.Failures) > 0 {
		// handle validation issue ...
	}
}
func extractTar(root string, tr io.Reader) error {
	return nil
}

func TestTar(t *testing.T) {
	/*
		data, err := makeTarStream()
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBuffer(data)
		str := NewTarStreamer(buf, append(DefaultKeywords, "sha1"))
	*/
	/*
		// open empty folder and check size.
		fh, err := os.Open("./testdata/empty")
		if err != nil {
			t.Fatal(err)
		}
		log.Println(fh.Stat())
		fh.Close() */
	fh, err := os.Open("./testdata/test.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, append(DefaultKeywords, "sha1"))

	if _, err := io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err := str.Close(); err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	// get DirectoryHierarcy struct from walking the tar archive
	tdh, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}
	if tdh == nil {
		t.Fatal("expected a DirectoryHierarchy struct, but got nil")
	}

	fh, err = os.Create("./testdata/test.mtree")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("./testdata/test.mtree")

	// put output of tar walk into test.mtree
	_, err = tdh.WriteTo(fh)
	if err != nil {
		t.Fatal(err)
	}
	fh.Close()

	// now simulate gomtree -T testdata/test.tar -f testdata/test.mtree
	fh, err = os.Open("./testdata/test.mtree")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	dh, err := ParseSpec(fh)
	if err != nil {
		t.Fatal(err)
	}

	res, err := TarCheck(tdh, dh, append(DefaultKeywords, "sha1"))

	if err != nil {
		t.Fatal(err)
	}

	// print any failures, and then call t.Fatal once all failures/extra/missing
	// are outputted
	if res != nil {
		errors := ""
		switch {
		case len(res.Failures) > 0:
			for _, f := range res.Failures {
				t.Errorf("%s\n", f)
			}
			errors += "Keyword validation errors\n"
		case len(res.Missing) > 0:
			for _, m := range res.Missing {
				missingpath, err := m.Path()
				if err != nil {
					t.Fatal(err)
				}
				t.Errorf("Missing file: %s\n", missingpath)
			}
			errors += "Missing files not expected for this test\n"
		case len(res.Extra) > 0:
			for _, e := range res.Extra {
				extrapath, err := e.Path()
				if err != nil {
					t.Fatal(err)
				}
				t.Errorf("Extra file: %s\n", extrapath)
			}
			errors += "Extra files not expected for this test\n"
		}
		if errors != "" {
			t.Fatal(errors)
		}
	}
}

// This test checks how gomtree handles archives that were created
// with multiple directories, i.e, archives created with something like:
// `tar -cvf some.tar dir1 dir2 dir3 dir4/dir5 dir6` ... etc.
// The testdata of collection.tar resemble such an archive. the `collection` folder
// is the contents of `collection.tar` extracted
func TestArchiveCreation(t *testing.T) {
	fh, err := os.Open("./testdata/collection.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, []string{"sha1"})

	if _, err := io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err := str.Close(); err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	// get DirectoryHierarcy struct from walking the tar archive
	tdh, err := str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}

	// Test the tar manifest against the actual directory
	res, err := Check("./testdata/collection", tdh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}

	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
		for _, e := range res.Extra {
			t.Errorf("%s extra not expected", e.Name)
		}
		for _, m := range res.Missing {
			t.Errorf("%s missing not expected", m.Name)
		}
	}

	// Test the tar manifest against itself
	res, err = TarCheck(tdh, tdh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
		for _, e := range res.Extra {
			t.Errorf("%s extra not expected", e.Name)
		}
		for _, m := range res.Missing {
			t.Errorf("%s missing not expected", m.Name)
		}
	}

	// Validate the directory manifest against the archive
	dh, err := Walk("./testdata/collection", nil, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	res, err = TarCheck(tdh, dh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
		for _, e := range res.Extra {
			t.Errorf("%s extra not expected", e.Name)
		}
		for _, m := range res.Missing {
			t.Errorf("%s missing not expected", m.Name)
		}
	}
}

// Now test a tar file that was created with just the path to a file. In this
// test case, the traversal and creation of "placeholder" directories are
// evaluated. Also, The fact that this archive contains a single entry, yet the
// entry is associated with a file that has parent directories, means that the
// "." directory should be the lowest sub-directory under which `file` is contained.
func TestTreeTraversal(t *testing.T) {
	fh, err := os.Open("./testdata/traversal.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, DefaultTarKeywords)

	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}

	fh.Close()
	tdh, err := str.Hierarchy()

	if err != nil {
		t.Fatal(err)
	}

	res, err := TarCheck(tdh, tdh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
		for _, e := range res.Extra {
			t.Errorf("%s extra not expected", e.Name)
		}
		for _, m := range res.Missing {
			t.Errorf("%s missing not expected", m.Name)
		}
	}

	// top-level "." directory will contain contents of traversal.tar
	res, err = Check("./testdata/.", tdh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
	}

	// Now test an archive that requires placeholder directories, i.e, there are
	// no headers in the archive that are associated with the actual directory name
	fh, err = os.Open("./testdata/singlefile.tar")
	if err != nil {
		t.Fatal(err)
	}
	str = NewTarStreamer(fh, DefaultTarKeywords)
	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}
	tdh, err = str.Hierarchy()
	if err != nil {
		t.Fatal(err)
	}

	// Implied top-level "." directory will contain the contents of singlefile.tar
	res, err = Check("./testdata/.", tdh, []string{"sha1"})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		for _, f := range res.Failures {
			t.Errorf(f.String())
		}
	}
}

func TestHardlinks(t *testing.T) {
	fh, err := os.Open("./testdata/hardlinks.tar")
	if err != nil {
		t.Fatal(err)
	}
	str := NewTarStreamer(fh, append(DefaultTarKeywords, "nlink"))

	if _, err = io.Copy(ioutil.Discard, str); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if err = str.Close(); err != nil {
		t.Fatal(err)
	}

	fh.Close()
	tdh, err := str.Hierarchy()

	if err != nil {
		t.Fatal(err)
	}
	foundnlink := false
	for _, e := range tdh.Entries {
		if e.Type == RelativeType {
			for _, kv := range e.Keywords {
				if KeyVal(kv).Keyword() == "nlink" {
					foundnlink = true
					if KeyVal(kv).Value() != "3" {
						t.Errorf("expected to have 3 hardlinks for %s", e.Name)
					}
				}
			}
		}
	}
	if !foundnlink {
		t.Errorf("nlink expected to be evaluated")
	}
}

// minimal tar archive stream that mimics what is in ./testdata/test.tar
func makeTarStream() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Add some files to the archive.
	var files = []struct {
		Name, Body string
		Mode       int64
		Type       byte
		Xattrs     map[string]string
	}{
		{"x/", "", 0755, '5', nil},
		{"x/files", "howdy\n", 0644, '0', nil},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name:   file.Name,
			Mode:   file.Mode,
			Size:   int64(len(file.Body)),
			Xattrs: file.Xattrs,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if len(file.Body) > 0 {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				return nil, err
			}
		}
	}
	// Make sure to check the error on Close.
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
