package api

import (
	"testing"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestFocusedRoutesRegisterOnlyAnthropicSurface(t *testing.T) {
	server := NewServer(
		&config.Config{Host: "127.0.0.1", Port: 8317},
		coreauth.NewManager(nil, nil, nil),
		sdkaccess.NewManager(),
		t.TempDir()+"/claudex.yaml",
		WithFocusedRoutes(),
	)

	got := make(map[string]struct{})
	for _, route := range server.engine.Routes() {
		got[route.Method+" "+route.Path] = struct{}{}
	}
	want := map[string]struct{}{
		"GET /v1/models":                 {},
		"POST /v1/messages":              {},
		"POST /v1/messages/count_tokens": {},
	}
	if len(got) != len(want) {
		t.Fatalf("focused routes = %#v, want %#v", got, want)
	}
	for route := range want {
		if _, ok := got[route]; !ok {
			t.Fatalf("focused routes missing %q: %#v", route, got)
		}
	}
}
