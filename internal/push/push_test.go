package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// decrypt is the user agent's half of RFC 8291, written from the RFC rather
// than from encrypt, so a shared mistake can't make the round trip pass.
func decrypt(t *testing.T, ua *ecdh.PrivateKey, auth, body []byte) []byte {
	t.Helper()
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	if rs != 4096 || idlen != 65 {
		t.Fatalf("header rs=%d idlen=%d", rs, idlen)
	}
	asPub, err := ecdh.P256().NewPublicKey(body[21 : 21+idlen])
	if err != nil {
		t.Fatal(err)
	}
	shared, _ := ua.ECDH(asPub)
	prkKey, _ := hkdf.Extract(sha256.New, shared, auth)
	ikm, _ := hkdf.Expand(sha256.New, prkKey, "WebPush: info\x00"+string(ua.PublicKey().Bytes())+string(asPub.Bytes()), 32)
	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	pt, err := gcm.Open(nil, nonce, body[21+idlen:], nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if pt[len(pt)-1] != 0x02 {
		t.Fatalf("missing last-record delimiter")
	}
	return pt[:len(pt)-1]
}

func TestSendEncryptsAndSigns(t *testing.T) {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	rand.Read(auth)

	got := make(chan *http.Request, 1)
	bodies := make(chan []byte, 1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- r
		bodies <- b
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	m, err := Open(filepath.Join(t.TempDir(), "push.json"), "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	m.client = srv.Client()
	if err := m.Subscribe(Subscription{Endpoint: srv.URL + "/push/abc", P256dh: b64.EncodeToString(ua.PublicKey().Bytes()), Auth: b64.EncodeToString(auth)}); err != nil {
		t.Fatal(err)
	}
	m.Send(Message{Title: "t", Body: "hello", Tag: "x"})

	var r *http.Request
	select {
	case r = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("no push request")
	}
	msg := decrypt(t, ua, auth, <-bodies)
	var back Message
	if err := json.Unmarshal(msg, &back); err != nil || back.Body != "hello" {
		t.Fatalf("payload %q", msg)
	}
	if r.Header.Get("Content-Encoding") != "aes128gcm" || len(r.Header.Get("Topic")) != 32 {
		t.Fatalf("headers %v", r.Header)
	}

	// VAPID: k is our public key and t verifies under it with the right audience.
	a := r.Header.Get("Authorization")
	if !strings.HasPrefix(a, "vapid t=") {
		t.Fatalf("authorization %q", a)
	}
	parts := strings.SplitN(strings.TrimPrefix(a, "vapid t="), ", k=", 2)
	if parts[1] != m.PublicKey() {
		t.Fatal("k is not our key")
	}
	seg := strings.Split(parts[0], ".")
	sig, _ := b64.DecodeString(seg[2])
	digest := sha256.Sum256([]byte(seg[0] + "." + seg[1]))
	if !ecdsa.Verify(&m.key.PublicKey, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("jwt signature does not verify")
	}
	var claims map[string]any
	cb, _ := b64.DecodeString(seg[1])
	json.Unmarshal(cb, &claims)
	if claims["aud"] != srv.URL || claims["sub"] != "mailto:test@example.com" {
		t.Fatalf("claims %v", claims)
	}
}

func TestKeyPersistsAndGoneSubscriptionIsDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "push.json")
	m, _ := Open(path, "mailto:x@y")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()
	m.client = srv.Client()
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	m.Subscribe(Subscription{Endpoint: srv.URL + "/p", P256dh: b64.EncodeToString(ua.PublicKey().Bytes()), Auth: b64.EncodeToString(make([]byte, 16))})

	again, err := Open(path, "mailto:x@y")
	if err != nil || again.PublicKey() != m.PublicKey() || again.Count() != 1 {
		t.Fatalf("reopen: err=%v same=%v count=%d", err, again.PublicKey() == m.PublicKey(), again.Count())
	}
	m.Send(Message{Title: "t"})
	for i := 0; i < 50 && m.Count() > 0; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if m.Count() != 0 {
		t.Fatal("410 subscription was kept")
	}
}

func TestSubscribeRejectsBadKeys(t *testing.T) {
	m, _ := Open(filepath.Join(t.TempDir(), "push.json"), "mailto:x@y")
	if m.Subscribe(Subscription{Endpoint: "http://insecure/x", P256dh: "", Auth: ""}) == nil {
		t.Fatal("accepted http endpoint")
	}
	if m.Subscribe(Subscription{Endpoint: "https://ok/x", P256dh: "AAAA", Auth: "AAAA"}) == nil {
		t.Fatal("accepted short keys")
	}
}

func TestSubscribeAgainKeepsDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "push.json")
	m, err := Open(path, "mailto:t@example.org")
	if err != nil {
		t.Fatal(err)
	}
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	sub := Subscription{Endpoint: "https://push.example/x", P256dh: b64.EncodeToString(ua.PublicKey().Bytes()), Auth: b64.EncodeToString(make([]byte, 16))}
	if err := m.Subscribe(sub); err != nil {
		t.Fatal(err)
	}
	first := m.List()[0].Added
	fi, _ := os.Stat(path)
	time.Sleep(20 * time.Millisecond)
	if err := m.Subscribe(sub); err != nil {
		t.Fatal(err)
	}
	if got := m.List(); len(got) != 1 || !got[0].Added.Equal(first) {
		t.Errorf("subscribing again: %d devices, added %v, want 1 and %v", len(got), got[0].Added, first)
	}
	if fi2, _ := os.Stat(path); !fi2.ModTime().Equal(fi.ModTime()) {
		t.Error("an unchanged subscription rewrote the file")
	}
	sub.Label = "Phone"
	if err := m.Subscribe(sub); err != nil {
		t.Fatal(err)
	}
	if got := m.List(); len(got) != 1 || got[0].Label != "Phone" || !got[0].Added.Equal(first) {
		t.Errorf("a new label: %+v", got)
	}
}
