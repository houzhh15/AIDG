package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMeetingOwnerInjectionRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	meetings := filepath.Join(root, "meetings")
	if err := os.MkdirAll(filepath.Join(meetings, "safe"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meetings, "safe", "metadata.json"), []byte(`{"owner":"alice"}`), 0o640); err != nil {
		t.Fatal(err)
	}
	injector := NewPermissionInjector(filepath.Join(root, "projects"), meetings)
	scopes, err := injector.InjectMeetingOwnerPermissions("alice", "safe", nil)
	if err != nil || len(scopes) != 2 {
		t.Fatalf("safe owner injection failed: %v %#v", err, scopes)
	}
	for _, id := range []string{"../safe", "../../users", "/absolute", ".."} {
		scopes, err = injector.InjectMeetingOwnerPermissions("alice", id, nil)
		if err != nil || len(scopes) != 0 {
			t.Fatalf("unsafe meeting ID %q injected permissions: %v %#v", id, err, scopes)
		}
		read, write, aclErr := injector.CheckMeetingACL("alice", id)
		if aclErr != nil || read || write {
			t.Fatalf("unsafe meeting ID %q injected ACL: %v read=%v write=%v", id, aclErr, read, write)
		}
	}
}

func TestMeetingPermissionFilesRejectSymlinks(t *testing.T) {
	root := t.TempDir()
	meetings := filepath.Join(root, "meetings")
	meetingDir := filepath.Join(meetings, "linked")
	if err := os.MkdirAll(meetingDir, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(outside, []byte(`{"owner":"alice"}`), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(meetingDir, "metadata.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	injector := NewPermissionInjector(filepath.Join(root, "projects"), meetings)
	scopes, _ := injector.InjectMeetingOwnerPermissions("alice", "linked", nil)
	if len(scopes) != 0 {
		t.Fatalf("symlink metadata granted scopes: %#v", scopes)
	}
}

func TestMeetingPermissionRejectsSymlinkMeetingDirectory(t *testing.T) {
	root := t.TempDir()
	meetings := filepath.Join(root, "meetings")
	if err := os.MkdirAll(meetings, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "metadata.json"), []byte(`{"owner":"alice"}`), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(meetings, "linked-dir")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	injector := NewPermissionInjector(filepath.Join(root, "projects"), meetings)
	scopes, _ := injector.InjectMeetingOwnerPermissions("alice", "linked-dir", nil)
	if len(scopes) != 0 {
		t.Fatalf("symlink meeting directory granted scopes: %#v", scopes)
	}
}
