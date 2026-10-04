package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saurlax/autodl-cli/internal/config"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	code := Execute(context.Background(), args, strings.NewReader(""), &out, &err, "test")
	return code, out.String(), err.String()
}

func TestHelpAndUsageDoNotRequireCredentials(t *testing.T) {
	t.Setenv("AUTODL_TOKEN", "")
	for _, tc := range []struct {
		args []string
		code int
		help string
	}{
		{nil, 0, "Available Commands:"},
		{[]string{"instance"}, 0, "release"},
		{[]string{"instance", "create"}, 2, "--gpu-spec"},
		{[]string{"instance", "status"}, 2, "INSTANCE_ID"},
		{[]string{"instance", "create", "--gpu-spec", "x", "--image", "y", "--cuda-min", "118", "--gpus", "5"}, 2, "--gpus"},
		{[]string{"instance", "list", "--page", "0"}, 2, "--page"},
		{[]string{"instance", "start", "x", "--unknown"}, 2, "--start-command"},
		{[]string{"instance", "release", "x"}, 2, "--yes"},
		{[]string{"image", "save", "x"}, 2, "--name"},
		{[]string{"storage", "mount", "--data-center", "x", "--type", "wrong"}, 2, "--type"},
		{[]string{"instance", "list", "unexpected"}, 2, "--page-size"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			code, out, err := run(t, tc.args...)
			if code != tc.code || !strings.Contains(out, tc.help) {
				t.Fatalf("code=%d out=%s err=%s", code, out, err)
			}
		})
	}
}

func TestDocumentedRequests(t *testing.T) {
	for _, tc := range []struct {
		args               []string
		method, path, body string
	}{
		{[]string{"account", "balance"}, "POST", "/api/v1/dev/wallet/balance", `{}`},
		{[]string{"instance", "list", "--page", "2", "--page-size", "7"}, "POST", "/api/v1/dev/instance/pro/list", `{"page_index":2,"page_size":7}`},
		{[]string{"instance", "get", "pro-test"}, "GET", "/api/v1/dev/instance/pro/snapshot", `{"instance_uuid":"pro-test"}`},
		{[]string{"instance", "status", "pro-test"}, "GET", "/api/v1/dev/instance/pro/status", `{"instance_uuid":"pro-test"}`},
		{[]string{"instance", "start", "pro-test", "--start-command", "echo ready"}, "POST", "/api/v1/dev/instance/pro/power_on", `{"instance_uuid":"pro-test","payload":"gpu","start_command":"echo ready"}`},
		{[]string{"instance", "stop", "pro-test"}, "POST", "/api/v1/dev/instance/pro/power_off", `{"instance_uuid":"pro-test"}`},
		{[]string{"instance", "release", "pro-test", "--yes"}, "POST", "/api/v1/dev/instance/pro/release", `{"instance_uuid":"pro-test"}`},
		{[]string{"image", "save", "pro-test", "--name", "my-image"}, "POST", "/api/v1/dev/instance/pro/image/save", `{"instance_uuid":"pro-test","image_name":"my-image"}`},
		{[]string{"image", "list"}, "POST", "/api/v1/dev/instance/pro/image/private/list", `{"page_index":1,"page_size":20}`},
		{[]string{"storage", "mount", "--data-center", "westDC2", "--type", "ordinary"}, "POST", "/api/v1/dev/exclusive_nfs/mount", `{"data_center":"westDC2","mountable":-1}`},
		{[]string{"instance", "create", "--gpu-spec", "new-spec", "--image", "image-test", "--cuda-min", "118", "--region", "westDC3,beijingDC2"}, "POST", "/api/v1/dev/instance/pro/create", `{"req_gpu_amount":1,"expand_system_disk_by_gb":0,"gpu_spec_uuid":"new-spec","image_uuid":"image-test","cuda_v_from":118,"data_center_list":["westDC3","beijingDC2"]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("got %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "test-token" {
					t.Errorf("incorrect auth header")
				}
				b, _ := io.ReadAll(r.Body)
				var got, want any
				if json.Unmarshal(b, &got) != nil || json.Unmarshal([]byte(tc.body), &want) != nil {
					t.Errorf("invalid body %s", b)
				}
				gb, _ := json.Marshal(got)
				wb, _ := json.Marshal(want)
				if !bytes.Equal(gb, wb) {
					t.Errorf("body got %s want %s", gb, wb)
				}
				io.WriteString(w, `{"code":"Success","data":null,"msg":"","request_id":"req-test"}`)
			}))
			defer server.Close()
			args := append(append([]string{}, tc.args...), "--base-url", server.URL, "--token", "test-token", "--json")
			code, out, err := run(t, args...)
			if code != 0 || !called || !strings.Contains(out, `"request_id":"req-test"`) {
				t.Fatalf("code=%d called=%v out=%s err=%s", code, called, out, err)
			}
		})
	}
}

func TestDryRunAndTokenPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, "stored"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTODL_TOKEN", "environment")
	o := options{configPath: path}
	for _, tc := range []struct{ flag, env, want string }{{"flag", "environment", "flag"}, {"", "environment", "environment"}, {"", "", "stored"}} {
		o.token = tc.flag
		t.Setenv("AUTODL_TOKEN", tc.env)
		got, err := o.resolveToken()
		if err != nil || got != tc.want {
			t.Fatalf("token=%s err=%v", got, err)
		}
	}
	code, out, err := run(t, "instance", "release", "pro-test", "--dry-run", "--config", filepath.Join(t.TempDir(), "missing"))
	if code != 0 || !strings.Contains(out, `"instance_uuid":"pro-test"`) || strings.Contains(out, "stored") {
		t.Fatalf("%d %s %s", code, out, err)
	}
	code, _, _ = run(t, "instance", "status", "x", "--base-url", "http://example.com", "--dry-run")
	if code != 2 {
		t.Fatalf("insecure origin accepted: %d", code)
	}
}

func TestHumanOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "balance") {
			io.WriteString(w, `{"code":"Success","data":{"assets":1234,"accumulate":123456789,"voucher_balance":-1}}`)
		} else {
			io.WriteString(w, `{"code":"Success","data":{"list":[{"uuid":"pro-test","name":"training","status":"running","gpu_spec_uuid":"h800","req_gpu_amount":1,"region_name":"Beijing"}],"page_index":1,"max_page":2,"result_total":21}}`)
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		args []string
		want []string
	}{{[]string{"account", "balance"}, []string{"1.234 CNY", "123456.789 CNY", "-0.001 CNY"}}, {[]string{"instance", "list"}, []string{"INSTANCE ID", "pro-test", "training", "Page 1/2 (21 records)"}}} {
		code, out, err := run(t, append(tc.args, "--token", "test", "--base-url", server.URL)...)
		if code != 0 {
			t.Fatal(err)
		}
		for _, s := range tc.want {
			if !strings.Contains(out, s) {
				t.Errorf("missing %s in %s", s, out)
			}
		}
	}
}
