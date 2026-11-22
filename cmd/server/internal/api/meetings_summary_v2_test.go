package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/meetingv2"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

type meetingACLInjector struct{ read, write bool }

func (m meetingACLInjector) InjectTaskOwnerPermissions(string, string, string, []string) ([]string, error) {
	return nil, nil
}
func (m meetingACLInjector) InjectMeetingOwnerPermissions(string, string, []string) ([]string, error) {
	return nil, nil
}
func (m meetingACLInjector) CheckMeetingACL(string, string) (bool, bool, error) {
	return m.read, m.write, nil
}

func TestAuthorizeMeetingV2TaskSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		user       string
		scopes     []string
		required   string
		want       bool
		wantStatus int
	}{
		{name: "owner", user: "alice", required: users.ScopeMeetingWrite, want: true},
		{name: "resource read", user: "bob", scopes: []string{users.ScopeMeetingRead}, required: users.ScopeMeetingRead, want: true},
		{name: "global scope is insufficient", user: "bob", required: users.ScopeMeetingRead, want: false, wantStatus: http.StatusForbidden},
		{name: "read cannot write", user: "bob", scopes: []string{users.ScopeMeetingRead}, required: users.ScopeMeetingWrite, want: false, wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Set("user", test.user)
			if test.scopes != nil {
				context.Set("meeting_resource_scopes", test.scopes)
			}
			got := authorizeMeetingV2TaskSession(context, meetingv2.Session{Owner: "alice"}, test.required)
			if got != test.want {
				t.Fatalf("authorized=%v, want %v", got, test.want)
			}
			if !got && context.Writer.Status() != test.wantStatus {
				t.Fatalf("status=%d, want %d", context.Writer.Status(), test.wantStatus)
			}
		})
	}
}

func TestAuthorizeMigratedLegacySessionUsesRouteScopeCompatibility(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("user", "legacy-reader")
	session := meetingv2.Session{ID: "legacy-session", Metadata: map[string]string{"migration_source": "legacy"}}
	if !authorizeMeetingV2TaskSession(context, session, users.ScopeMeetingRead) {
		t.Fatal("migrated legacy session lost historical route-scope access")
	}
}

func TestAuthorizeMeetingV2TaskSessionUsesResolvedACL(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("user", "bob")
	if !authorizeMeetingV2TaskSession(context, meetingv2.Session{ID: "meeting-1", Owner: "alice"}, users.ScopeMeetingRead, meetingACLInjector{read: true}) {
		t.Fatal("expected ACL read access")
	}
	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Set("user", "bob")
	if authorizeMeetingV2TaskSession(context, meetingv2.Session{ID: "meeting-1", Owner: "alice"}, users.ScopeMeetingWrite, meetingACLInjector{read: true}) {
		t.Fatal("read ACL unexpectedly granted write")
	}
}
