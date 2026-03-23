package jmespath

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParser(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("foo"),
		[]byte("foo.bar"),
		[]byte("[*]"),
		[]byte("length(foo)"),
		[]byte("foo[0:2]"),
	} {
		f.Add(seed)
	}
	if fuzzFlag := flag.Lookup("test.fuzz"); fuzzFlag != nil && fuzzFlag.Value.String() != "" {
		addLegacyCorpusSeeds(f, filepath.Join("fuzz", "testdata"))
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		p := NewParser()
		_, _ = p.Parse(string(data))
	})
}

func addLegacyCorpusSeeds(f *testing.F, root string) {
	if _, err := os.Stat(root); err != nil {
		return
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		seed, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		f.Add(seed)
		return nil
	})
}
