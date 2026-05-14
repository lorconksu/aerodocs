package terminal

import (
	"fmt"
	osuser "os/user"
	"reflect"
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
