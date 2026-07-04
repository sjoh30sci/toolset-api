package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(lightURL, heavyURL string, lightOn, heavyOn bool) *Client {
	c := NewClient()
	c.LightURL = lightURL
	c.HeavyURL = heavyURL
	c.LightEnabled = lightOn
	c.HeavyEnabled = heavyOn
	c.HTTP = &http.Client{Timeout: 5 * time.Second}
	return c
}

func TestExecuteProxiesToLight(t *testing.T) {
	got := ""
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","exit_code":0,"stdout":"ok"}`))
	}))
	defer mock.Close()

	c := testClient(mock.URL, "", true, false)
	res, err := c.Execute(context.Background(), ExecRequest{Code: "print(1)", Language: "python"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "/exec" {
		t.Errorf("expected POST to /exec, got %s", got)
	}
	if res.Status != "success" || res.Stdout != "ok" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestExecuteUnsupportedLanguage(t *testing.T) {
	c := testClient("http://unused", "", true, false)
	_, err := c.Execute(context.Background(), ExecRequest{Code: "x", Language: "cobol"})
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestExecuteTierDisabled(t *testing.T) {
	c := testClient("http://unused", "http://unused", true, false)
	_, err := c.Execute(context.Background(), ExecRequest{Code: "x", Language: "java"})
	if err == nil {
		t.Fatal("expected error when heavy tier disabled")
	}
}

func TestExecuteUpstreamDown(t *testing.T) {
	c := testClient("http://127.0.0.1:0", "", true, false)
	_, err := c.Execute(context.Background(), ExecRequest{Code: "print(1)", Language: "python"})
	if err == nil {
		t.Fatal("expected error when upstream unreachable")
	}
}

func TestExecuteBadStatus(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer mock.Close()

	c := testClient(mock.URL, "", true, false)
	_, err := c.Execute(context.Background(), ExecRequest{Code: "print(1)", Language: "python"})
	if err == nil {
		t.Fatal("expected error on 500 upstream status")
	}
}

func TestURLForLanguage(t *testing.T) {
	c := testClient("http://light", "http://heavy", true, true)
	cases := map[string]string{
		"python": "http://light",
		"bash":   "http://light",
		"java":   "http://heavy",
		"rust":   "http://heavy",
		"dotnet": "http://heavy",
	}
	for lang, want := range cases {
		got, err := c.URLForLanguage(lang)
		if err != nil {
			t.Errorf("%s: unexpected error %v", lang, err)
			continue
		}
		if got != want {
			t.Errorf("%s: want %s, got %s", lang, want, got)
		}
	}
}
