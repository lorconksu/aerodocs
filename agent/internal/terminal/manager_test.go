package terminal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	osuser "os/user"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

func TestExecutionIdentityForRunAsBlankUsesAgentUser(t *testing.T) {
	identity, err := executionIdentityForRunAs("")
	if err != nil {
		t.Fatalf("executionIdentityForRunAs blank returned error: %v", err)
	}
	if identity != nil {
		t.Fatalf("expected blank run-as user to use current agent identity, got %+v", identity)
	}
}

func TestExecutionIdentityForRunAsUnknownUser(t *testing.T) {
	_, err := executionIdentityForRunAs("aerodocs-definitely-missing-user")
	if err == nil {
		t.Fatal("expected unknown run-as user to fail")
	}
	if !strings.Contains(err.Error(), "terminal execution user not available") {
		t.Fatalf("expected unavailable-user error, got %v", err)
	}
}

func TestExecutionIdentityForRunAsRejectsUnsafeUsername(t *testing.T) {
	for _, username := range []string{"-alice", "alice/root", "alice:staff", "alice staff", "alice\nstaff"} {
		_, err := executionIdentityForRunAs(username)
		if err == nil {
			t.Fatalf("expected username %q to be rejected", username)
		}
		if !strings.Contains(err.Error(), "invalid terminal execution user") {
			t.Fatalf("expected invalid-user error for %q, got %v", username, err)
		}
	}
}

func TestExecutionIdentityForRunAsFallsBackToNSS(t *testing.T) {
	originalLookupOSUser := lookupOSUser
	originalLookupNSSUser := lookupNSSUser
	originalLookupNSSGroupIDs := lookupNSSGroupIDs
	t.Cleanup(func() {
		lookupOSUser = originalLookupOSUser
		lookupNSSUser = originalLookupNSSUser
		lookupNSSGroupIDs = originalLookupNSSGroupIDs
	})

	lookupOSUser = func(string) (*osuser.User, error) {
		return nil, fmt.Errorf("not found in os/user")
	}
	lookupNSSUser = func(username string) (*osuser.User, error) {
		if username != "ldapuser" {
			t.Fatalf("unexpected username %q", username)
		}
		return &osuser.User{
			Username: "ldapuser",
			Uid:      "1545800010",
			Gid:      "1545800010",
			Name:     "LDAP User",
			HomeDir:  "/home/ldapuser",
		}, nil
	}
	lookupNSSGroupIDs = func(username string) ([]string, error) {
		if username != "ldapuser" {
			t.Fatalf("unexpected username %q", username)
		}
		return []string{"1545800010", "1545800009", "1545800008"}, nil
	}

	identity, err := executionIdentityForRunAs("ldapuser")
	if err != nil {
		t.Fatalf("executionIdentityForRunAs: %v", err)
	}
	if identity.username != "ldapuser" {
		t.Fatalf("username = %q, want ldapuser", identity.username)
	}
	if identity.homeDir != "/home/ldapuser" {
		t.Fatalf("homeDir = %q, want /home/ldapuser", identity.homeDir)
	}
	if identity.credential.Uid != 1545800010 || identity.credential.Gid != 1545800010 {
		t.Fatalf("credential uid/gid = %d/%d", identity.credential.Uid, identity.credential.Gid)
	}
	wantGroups := []uint32{1545800010, 1545800009, 1545800008}
	if !reflect.DeepEqual(identity.credential.Groups, wantGroups) {
		t.Fatalf("groups = %#v, want %#v", identity.credential.Groups, wantGroups)
	}
}

func TestBuildTerminalEnvDoesNotInheritProcessSecrets(t *testing.T) {
	t.Setenv("AERODOCS_AGENT_SECRET", "do-not-leak")
	env := buildTerminalEnv("/bin/bash", &executionIdentity{
		username: "ldapuser",
		homeDir:  "/home/ldapuser",
	})

	for _, entry := range env {
		if strings.HasPrefix(entry, "AERODOCS_AGENT_SECRET=") {
			t.Fatalf("terminal env inherited process secret: %v", env)
		}
	}
	if !containsEnv(env, "USER=ldapuser") || !containsEnv(env, "HOME=/home/ldapuser") || !containsEnv(env, "SHELL=/bin/bash") {
		t.Fatalf("terminal env missing expected identity values: %v", env)
	}
}

func TestSendUnblocksAfterCloseAll(t *testing.T) {
	m := NewManager(make(chan *pb.AgentMessage))
	result := make(chan bool, 1)

	go func() {
		result <- m.Send(&pb.AgentMessage{})
	}()

	m.CloseAll()

	select {
	case sent := <-result:
		if sent {
			t.Fatal("expected send to stop after manager shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("send did not unblock after manager shutdown")
	}
}

func TestOpenAfterCloseAllRejected(t *testing.T) {
	m := NewManager(make(chan *pb.AgentMessage, 1))
	m.CloseAll()

	err := m.Open("session-1", 80, 24, "", "")
	if err == nil || !strings.Contains(err.Error(), "terminal manager unavailable") {
		t.Fatalf("expected stopped manager error, got %v", err)
	}
}

func TestManagerOpenInputResizeAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty is not supported on windows")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is required for terminal lifecycle test")
	}
	t.Setenv("SHELL", shell)

	sendCh := make(chan *pb.AgentMessage, 16)
	m := NewManager(sendCh)
	t.Cleanup(m.CloseAll)

	cwd := t.TempDir()
	if err := m.Open("session-pty", 80, 24, cwd, ""); err != nil {
		t.Fatalf("open terminal: %v", err)
	}
	if err := m.Resize("session-pty", 100, 32); err != nil {
		t.Fatalf("resize terminal: %v", err)
	}
	if err := m.Input("session-pty", []byte("printf aerodocs-ready\\n; exit\n")); err != nil {
		t.Fatalf("write terminal input: %v", err)
	}

	var data bytes.Buffer
	sawExit := false
	deadline := time.After(3 * time.Second)
	for !sawExit {
		select {
		case msg := <-sendCh:
			if payload := msg.GetTerminalData(); payload != nil {
				data.Write(payload.Data)
			}
			if payload := msg.GetTerminalExit(); payload != nil {
				if payload.SessionId != "session-pty" {
					t.Fatalf("exit session id = %q", payload.SessionId)
				}
				if payload.ExitCode != 0 {
					t.Fatalf("exit code = %d, error = %q", payload.ExitCode, payload.Error)
				}
				sawExit = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for terminal exit; output so far: %q", data.String())
		}
	}
	if !strings.Contains(data.String(), "aerodocs-ready") {
		t.Fatalf("expected command output, got %q", data.String())
	}
	if _, err := m.get("session-pty"); !errorsIsSessionNotFound(err) {
		t.Fatalf("expected session cleanup after exit, got %v", err)
	}
}

func TestManagerOpenRejectsInvalidCwd(t *testing.T) {
	m := NewManager(make(chan *pb.AgentMessage, 1))
	err := m.Open("session-bad-cwd", 80, 24, "relative/path", "")
	if err == nil || !strings.Contains(err.Error(), "terminal cwd must be absolute") {
		t.Fatalf("expected absolute cwd error, got %v", err)
	}

	filePath := t.TempDir() + "/not-a-dir"
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write cwd file fixture: %v", err)
	}
	err = m.Open("session-file-cwd", 80, 24, filePath, "")
	if err == nil || !strings.Contains(err.Error(), "terminal cwd must be a directory") {
		t.Fatalf("expected directory cwd error, got %v", err)
	}
}

func TestManagerOpenRejectsDuplicateSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty is not supported on windows")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is required for duplicate session test")
	}
	t.Setenv("SHELL", shell)

	m := NewManager(make(chan *pb.AgentMessage, 16))
	t.Cleanup(m.CloseAll)
	if err := m.Open("duplicate", 80, 24, "", ""); err != nil {
		t.Fatalf("open first terminal: %v", err)
	}
	if err := m.Open("duplicate", 80, 24, "", ""); err == nil || !strings.Contains(err.Error(), "terminal session already exists") {
		t.Fatalf("expected duplicate session error, got %v", err)
	}
}

func TestNormalizedSizeBounds(t *testing.T) {
	size := normalizedSize(0, 0)
	if size.Cols != defaultCols || size.Rows != defaultRows {
		t.Fatalf("default size = %dx%d", size.Cols, size.Rows)
	}
	size = normalizedSize(maxCols+1, maxRows+1)
	if size.Cols != maxCols || size.Rows != maxRows {
		t.Fatalf("bounded size = %dx%d", size.Cols, size.Rows)
	}
}

func TestExecutionIdentityForRunAsRejectsInvalidIDs(t *testing.T) {
	originalLookupOSUser := lookupOSUser
	originalLookupOSUserGroups := lookupOSUserGroups
	originalLookupNSSUser := lookupNSSUser
	originalLookupNSSGroupIDs := lookupNSSGroupIDs
	t.Cleanup(func() {
		lookupOSUser = originalLookupOSUser
		lookupOSUserGroups = originalLookupOSUserGroups
		lookupNSSUser = originalLookupNSSUser
		lookupNSSGroupIDs = originalLookupNSSGroupIDs
	})

	lookupOSUser = func(string) (*osuser.User, error) {
		return &osuser.User{Username: "ldapuser", Uid: "not-a-uid", Gid: "1545800010"}, nil
	}
	lookupOSUserGroups = func(*osuser.User) ([]string, error) {
		return []string{"1545800010"}, nil
	}
	_, err := executionIdentityForRunAs("ldapuser")
	if err == nil || !strings.Contains(err.Error(), "invalid uid") {
		t.Fatalf("expected invalid uid error, got %v", err)
	}

	lookupOSUser = func(string) (*osuser.User, error) {
		return &osuser.User{Username: "ldapuser", Uid: "1545800010", Gid: "not-a-gid"}, nil
	}
	_, err = executionIdentityForRunAs("ldapuser")
	if err == nil || !strings.Contains(err.Error(), "invalid gid") {
		t.Fatalf("expected invalid gid error, got %v", err)
	}

	lookupOSUser = func(string) (*osuser.User, error) {
		return &osuser.User{Username: "ldapuser", Uid: "1545800010", Gid: "1545800010"}, nil
	}
	lookupOSUserGroups = func(*osuser.User) ([]string, error) {
		return []string{"not-a-group"}, nil
	}
	_, err = executionIdentityForRunAs("ldapuser")
	if err == nil || !strings.Contains(err.Error(), "invalid group") {
		t.Fatalf("expected invalid group error, got %v", err)
	}
}

func TestParseGetentPasswdLine(t *testing.T) {
	u, err := parseGetentPasswdLine("ldapuser:*:1545800010:1545800010:LDAP User:/home/ldapuser:/bin/bash")
	if err != nil {
		t.Fatalf("parseGetentPasswdLine: %v", err)
	}
	if u.Username != "ldapuser" || u.Uid != "1545800010" || u.Gid != "1545800010" || u.Name != "LDAP User" || u.HomeDir != "/home/ldapuser" {
		t.Fatalf("unexpected parsed user: %#v", u)
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func errorsIsSessionNotFound(err error) bool {
	return err == ErrSessionNotFound
}

func TestParseGroupIDs(t *testing.T) {
	groupIDs, err := parseGroupIDs("1545800010 1545800009 1545800008\n")
	if err != nil {
		t.Fatalf("parseGroupIDs: %v", err)
	}
	want := []string{"1545800010", "1545800009", "1545800008"}
	if !reflect.DeepEqual(groupIDs, want) {
		t.Fatalf("groupIDs = %#v, want %#v", groupIDs, want)
	}
}
