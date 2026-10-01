// Command fcmsend sends a test message to one device through the FCM HTTP
// v1 API, authenticated with a Firebase service account key. Standard
// library only.
//
//	go run ./tools/fcmsend -key ~/.config/drift/fcm-sa.json -token <FCM token> \
//	    -title Hello -body World -data route=/inbox
//
// -data-only sends a data message (no notification): it reaches the app's
// Messages stream while the app runs, in the foreground or background, and
// is never shown by the system.
package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type dataFlag map[string]string

func (d dataFlag) String() string { return fmt.Sprint(map[string]string(d)) }

func (d dataFlag) Set(kv string) error {
	k, v, ok := strings.Cut(kv, "=")
	if !ok || k == "" {
		return fmt.Errorf("want key=value, got %q", kv)
	}
	d[k] = v
	return nil
}

// serviceAccount is the part of a service account key file fcmsend uses.
type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func main() {
	keyPath := flag.String("key", os.Getenv("DRIFT_FCM_KEY"), "service account key JSON (default $DRIFT_FCM_KEY)")
	token := flag.String("token", "", "the device's FCM registration token")
	title := flag.String("title", "Drift", "notification title")
	body := flag.String("body", "Hello from fcmsend", "notification body")
	dataOnly := flag.Bool("data-only", false, "send a data message without a notification")
	data := dataFlag{}
	flag.Var(data, "data", "custom data key=value (repeatable)")
	flag.Parse()

	if *keyPath == "" || *token == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*keyPath, *token, *title, *body, *dataOnly, data); err != nil {
		fmt.Fprintln(os.Stderr, "fcmsend:", err)
		os.Exit(1)
	}
}

func run(keyPath, token, title, body string, dataOnly bool, data map[string]string) error {
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return fmt.Errorf("parse %s: %w", keyPath, err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" || sa.TokenURI == "" {
		return fmt.Errorf("%s is not a service account key", keyPath)
	}
	access, err := accessToken(sa)
	if err != nil {
		return err
	}

	message := map[string]any{"token": token}
	if len(data) > 0 {
		message["data"] = data
	}
	if dataOnly {
		// Wake the app: iOS needs a background push, Android high priority.
		message["apns"] = map[string]any{
			"headers": map[string]string{"apns-push-type": "background", "apns-priority": "5"},
			"payload": map[string]any{"aps": map[string]any{"content-available": 1}},
		}
		message["android"] = map[string]any{"priority": "high"}
	} else {
		message["notification"] = map[string]string{"title": title, "body": body}
	}
	payload, _ := json.Marshal(map[string]any{"message": message})

	endpoint := "https://fcm.googleapis.com/v1/projects/" + sa.ProjectID + "/messages:send"
	req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send: %s: %s", resp.Status, out)
	}
	fmt.Printf("sent: %s\n", out)
	return nil
}

// accessToken exchanges a signed JWT for an OAuth2 access token with the
// Firebase Messaging scope (RFC 7523, as Google's client libraries do).
func accessToken(sa serviceAccount) (string, error) {
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", errors.New("service account private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private_key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("service account private_key is not RSA")
	}

	now := time.Now()
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	unsigned := header + "." + enc.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	assertion := unsigned + "." + enc.EncodeToString(sig)

	resp, err := http.PostForm(sa.TokenURI, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &tok) != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("token exchange: %s: %s", resp.Status, body)
	}
	return tok.AccessToken, nil
}
