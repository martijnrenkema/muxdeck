// Package push delivers Web Push notifications (RFC 8030) so agent alerts
// reach a phone whose muxdeck PWA is closed. Payloads are encrypted per
// RFC 8291 (aes128gcm) and requests signed with VAPID (RFC 8292), all on the
// standard library: the push service only ever sees ciphertext.
package push

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var b64 = base64.RawURLEncoding

// Subscription is what PushManager.subscribe() hands the browser, flattened.
type Subscription struct {
	Endpoint string    `json:"endpoint"`
	P256dh   string    `json:"p256dh"`
	Auth     string    `json:"auth"`
	Label    string    `json:"label,omitempty"`
	Added    time.Time `json:"added"`
}

// Device is a subscription as the settings panel lists it.
type Device struct {
	Endpoint string    `json:"endpoint"`
	Label    string    `json:"label"`
	Added    time.Time `json:"added"`
}

// Message is the JSON the service worker receives and turns into a
// notification. Actions become notification buttons; the worker knows what
// each one does.
type Message struct {
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Session string   `json:"session,omitempty"`
	Tag     string   `json:"tag,omitempty"`
	Perm    string   `json:"perm,omitempty"`
	Actions []string `json:"actions,omitempty"`
	// Titles labels the actions where the worker has no name for them
	// (the options of a question).
	Titles []string `json:"titles,omitempty"`
	// Block is the id of an action auto mode blocked; its buttons answer it.
	Block string `json:"block,omitempty"`
	// Expires (Unix ms) marks a notification that is useless afterwards,
	// such as a permission request nobody waits on any more.
	Expires int64 `json:"expires,omitempty"`
	// TTL is how long the push service keeps an undelivered message
	// (default one hour).
	TTL time.Duration `json:"-"`
}

type state struct {
	VAPID string         `json:"vapid_private"`
	Subs  []Subscription `json:"subscriptions"`
}

type Manager struct {
	path    string
	subject string
	client  *http.Client

	mu   sync.Mutex
	key  *ecdsa.PrivateKey
	subs []Subscription
}

// Open loads or creates the key and subscription file. The VAPID key is
// generated once and kept: a new key orphans every existing subscription.
func Open(path, subject string) (*Manager, error) {
	m := &Manager{path: path, subject: subject, client: &http.Client{Timeout: 15 * time.Second}}
	var st state
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &st); err != nil {
			return nil, fmt.Errorf("push: %s: %w", path, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	if st.VAPID != "" {
		der, err := b64.DecodeString(st.VAPID)
		if err != nil {
			return nil, fmt.Errorf("push: vapid key: %w", err)
		}
		k, err := x509.ParsePKCS8PrivateKey(der)
		if err != nil {
			return nil, fmt.Errorf("push: vapid key: %w", err)
		}
		ek, ok := k.(*ecdsa.PrivateKey)
		if !ok || ek.Curve != elliptic.P256() {
			return nil, errors.New("push: vapid key is not P-256")
		}
		m.key = ek
	} else {
		if m.key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
			return nil, err
		}
	}
	m.subs = st.Subs
	if err := m.save(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) save() error {
	der, err := x509.MarshalPKCS8PrivateKey(m.key)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state{VAPID: b64.EncodeToString(der), Subs: m.subs}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}
	return writeFileSync(m.path, data)
}

// writeFileSync replaces path with data so that a crash leaves either the
// old file or the new one, never an empty one: the VAPID key in it cannot
// be made again without orphaning every subscription.
func writeFileSync(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if serr := f.Sync(); err == nil {
		err = serr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// WriteFileSync is writeFileSync for the daemon's other small state files.
func WriteFileSync(path string, data []byte) error { return writeFileSync(path, data) }

// PublicKey is the applicationServerKey the browser subscribes with.
func (m *Manager) PublicKey() string {
	pub, _ := m.key.PublicKey.ECDH()
	return b64.EncodeToString(pub.Bytes())
}

func (m *Manager) List() []Device {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Device, 0, len(m.subs))
	for _, s := range m.subs {
		out = append(out, Device{Endpoint: s.Endpoint, Label: s.Label, Added: s.Added})
	}
	return out
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subs)
}

// Subscribe adds or refreshes a subscription; the endpoint is its identity.
func (m *Manager) Subscribe(sub Subscription) error {
	if u, err := url.Parse(sub.Endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("endpoint must be an https URL")
	}
	if p, err := b64.DecodeString(sub.P256dh); err != nil || len(p) != 65 {
		return errors.New("bad p256dh")
	}
	if a, err := b64.DecodeString(sub.Auth); err != nil || len(a) != 16 {
		return errors.New("bad auth")
	}
	sub.Added = time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.subs[:0]
	for _, s := range m.subs {
		if s.Endpoint != sub.Endpoint {
			kept = append(kept, s)
			continue
		}
		// A browser signs up again on every page load: that is the same
		// device, added when it first came.
		if !s.Added.IsZero() {
			sub.Added = s.Added
		}
		if s == sub {
			return nil
		}
	}
	m.subs = append(kept, sub)
	return m.save()
}

func (m *Manager) Unsubscribe(endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.subs[:0]
	for _, s := range m.subs {
		if s.Endpoint != endpoint {
			kept = append(kept, s)
		}
	}
	m.subs = kept
	return m.save()
}

// Send delivers msg to every subscription in the background. A push service
// answering 404 or 410 has dropped the subscription for good, so it goes.
func (m *Manager) Send(msg Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	m.mu.Lock()
	subs := append([]Subscription(nil), m.subs...)
	m.mu.Unlock()
	for _, sub := range subs {
		go func() {
			status, err := m.deliver(sub, payload, msg.Tag, msg.TTL)
			switch {
			case err != nil:
				log.Printf("push: %s: %v", host(sub.Endpoint), err)
			case status == http.StatusNotFound || status == http.StatusGone:
				log.Printf("push: %s: subscription gone, removing", host(sub.Endpoint))
				m.Unsubscribe(sub.Endpoint)
			case status >= 300:
				log.Printf("push: %s: status %d", host(sub.Endpoint), status)
			}
		}()
	}
}

func host(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return u.Host
	}
	return "?"
}

func (m *Manager) deliver(sub Subscription, payload []byte, topic string, ttl time.Duration) (int, error) {
	body, err := encrypt(sub, payload)
	if err != nil {
		return 0, err
	}
	jwt, err := m.vapidJWT(sub.Endpoint)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	if ttl <= 0 {
		ttl = time.Hour
	}
	req.Header.Set("TTL", strconv.Itoa(int(ttl/time.Second)))
	req.Header.Set("Urgency", "high")
	if topic != "" {
		// A newer message with the same topic replaces an undelivered older
		// one, so a phone that was offline gets the latest state, not a backlog.
		req.Header.Set("Topic", topicSafe(topic))
	}
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+m.PublicKey())
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return resp.StatusCode, nil
}

// topicSafe maps a tag onto the 32-char URL-safe base64 alphabet a Topic
// header allows.
func topicSafe(tag string) string {
	sum := sha256.Sum256([]byte(tag))
	return b64.EncodeToString(sum[:])[:32]
}

func (m *Manager) vapidJWT(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	header := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		"exp": time.Now().Add(12 * time.Hour).Unix(),
		"sub": m.subject,
	})
	signing := header + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, m.key, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signing + "." + b64.EncodeToString(sig), nil
}

// encrypt builds an RFC 8291 aes128gcm body: a header carrying the salt and
// the sender's ephemeral public key, then one record of ciphertext.
func encrypt(sub Subscription, plaintext []byte) ([]byte, error) {
	uaBytes, err := b64.DecodeString(sub.P256dh)
	if err != nil {
		return nil, err
	}
	authSecret, err := b64.DecodeString(sub.Auth)
	if err != nil {
		return nil, err
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaBytes)
	if err != nil {
		return nil, err
	}
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	asPub := asPriv.PublicKey().Bytes()
	shared, err := asPriv.ECDH(uaPub)
	if err != nil {
		return nil, err
	}

	prkKey, err := hkdf.Extract(sha256.New, shared, authSecret)
	if err != nil {
		return nil, err
	}
	keyInfo := "WebPush: info\x00" + string(uaBytes) + string(asPub)
	ikm, err := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// 0x02 marks the last (and only) record.
	ct := gcm.Seal(nil, nonce, append(append([]byte(nil), plaintext...), 0x02), nil)

	const recordSize = 4096
	if len(ct)+86 > recordSize {
		return nil, errors.New("payload too large")
	}
	out := make([]byte, 0, 16+4+1+len(asPub)+len(ct))
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, recordSize)
	out = append(out, byte(len(asPub)))
	out = append(out, asPub...)
	return append(out, ct...), nil
}
