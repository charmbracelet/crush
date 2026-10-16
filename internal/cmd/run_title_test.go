package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestResolveSessionTitle(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, id, prompt, want string
		last                   bool
	}{
		{name: "new", prompt: "  Fix\n login  ", want: "Fix login"},
		{name: "empty", prompt: " \n\t", want: "non-interactive"},
		{name: "resume", id: "existing", prompt: "Different task", want: "Existing title"},
		{name: "last", last: true, prompt: "Different task", want: "Existing title"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var created string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					var s proto.Session
					if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					created = s.Title
					s.ID = "created"
					_ = json.NewEncoder(w).Encode(s)
				case http.MethodGet:
					s := proto.Session{ID: "existing", Title: "Existing title"}
					if tt.last {
						_ = json.NewEncoder(w).Encode([]proto.Session{s})
					} else {
						_ = json.NewEncoder(w).Encode(s)
					}
				}
			}))
			defer srv.Close()
			c, err := client.NewClient(t.TempDir(), "tcp", srv.Listener.Addr().String())
			require.NoError(t, err)
			sess, err := resolveSession(t.Context(), c, "workspace", tt.id, tt.last, tt.prompt)
			require.NoError(t, err)
			require.Equal(t, tt.want, sess.Title)
			if tt.last || tt.id != "" {
				require.Empty(t, created)
			} else {
				require.Equal(t, tt.want, created)
			}
		})
	}
}
