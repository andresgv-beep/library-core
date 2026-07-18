package download

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestRedirectPolicyRejectsInternalTargets(t *testing.T) {
	client := newSafeHTTPClient()
	for _, raw := range []string{
		"http://127.0.0.1/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/private",
		"file:///etc/passwd",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		req := &http.Request{URL: u}
		if err := client.CheckRedirect(req, []*http.Request{{}}); err == nil {
			t.Errorf("permitió redirect interno %s", raw)
		}
	}
}

func TestSafeDialRejectsPrivateAddressBeforeConnecting(t *testing.T) {
	if _, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("safeDialContext permitió loopback")
	}
	if !downloadIPIsPublic([]byte{93, 184, 216, 34}) {
		t.Fatal("clasificó una IPv4 pública como interna")
	}
}
