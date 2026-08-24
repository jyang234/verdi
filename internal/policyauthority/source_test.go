package policyauthority

import (
	"os"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestLoadFromSourceEquivalentToFilesystem(t *testing.T) {
	root := t.TempDir()
	files := minimalStoreFiles()
	writeTree(t, root, files)
	source := make(fstest.MapFS, len(files))
	for name, data := range files {
		source[name] = &fstest.MapFile{Data: []byte(data)}
	}

	fromDisk, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	fromSource, err := LoadFromSource(source)
	if err != nil {
		t.Fatalf("LoadFromSource() error = %v", err)
	}
	diskProfile, err := fromDisk.SelectedProfile()
	if err != nil {
		t.Fatalf("disk SelectedProfile() error = %v", err)
	}
	sourceProfile, err := fromSource.SelectedProfile()
	if err != nil {
		t.Fatalf("source SelectedProfile() error = %v", err)
	}
	diskDigest, err := diskProfile.Digest()
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := sourceProfile.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if diskDigest != sourceDigest || !reflect.DeepEqual(fromDisk.Policies, fromSource.Policies) {
		t.Fatalf("source-backed store differs from filesystem store: profile %q/%q", diskDigest, sourceDigest)
	}
}

func TestLoadFromSourceFailuresStayOperational(t *testing.T) {
	if _, err := LoadFromSource(fstest.MapFS{}); err == nil {
		t.Fatal("LoadFromSource(empty) error = nil")
	}
	if _, err := LoadFromSource(nil); err == nil {
		t.Fatal("LoadFromSource(nil) error = nil")
	}
	root := t.TempDir()
	if err := os.MkdirAll(root+"/.verdi/policy", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("Load(incomplete) error = nil")
	}
}

func TestSelectedProfileDeepCloneAndSeal(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())
	store, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.SelectedProfile()
	if err != nil {
		t.Fatalf("SelectedProfile() error = %v", err)
	}
	first.RoleMappings[0].Subjects[0] = "mutated"
	if _, err := first.Digest(); err == nil {
		t.Fatal("mutated returned profile retained a valid seal")
	}
	second, err := store.SelectedProfile()
	if err != nil {
		t.Fatalf("SelectedProfile() after returned-copy mutation error = %v", err)
	}
	if second.RoleMappings[0].Subjects[0] == "mutated" {
		t.Fatal("SelectedProfile() shared nested subjects with a prior result")
	}
	if _, err := second.Digest(); err != nil {
		t.Fatalf("SelectedProfile() returned an unsealed copy: %v", err)
	}

	store.Profiles[store.Constitution.SelectedProfile].Profile.RoleMappings[0].Subjects[0] = "tampered"
	if _, err := store.SelectedProfile(); err == nil {
		t.Fatal("SelectedProfile() accepted a post-load mutated stored profile")
	}
	if _, err := (&Store{}).SelectedProfile(); err == nil {
		t.Fatal("zero Store.SelectedProfile() error = nil")
	}
}
