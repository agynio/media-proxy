package httpauth

import "testing"

func TestExtractBearerToken(t *testing.T) {
	token, ok := ExtractBearerToken("Bearer sample-token")
	if !ok {
		t.Fatalf("expected token to be extracted")
	}
	if token != "sample-token" {
		t.Fatalf("unexpected token: %s", token)
	}

	if _, ok := ExtractBearerToken(""); ok {
		t.Fatalf("expected empty header to be rejected")
	}

	if _, ok := ExtractBearerToken("Basic abc123"); ok {
		t.Fatalf("expected non-bearer header to be rejected")
	}

	if _, ok := ExtractBearerToken("Bearer "); ok {
		t.Fatalf("expected empty bearer token to be rejected")
	}
}
