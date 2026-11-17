package syncer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ArtifactResolver reports where the current content tarball lives and which
// revision it holds.
type ArtifactResolver func(ctx context.Context) (url, revision string, err error)

const (
	serviceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount"
	gitRepositoryPath = "/apis/source.toolkit.fluxcd.io/v1/namespaces/%s/gitrepositories/%s"
)

// GitRepositoryResolver reads status.artifact of a Flux GitRepository from the
// Kubernetes API. source-controller names each tarball after its revision and
// serves no stable "latest" path, so the current URL has to be looked up.
func GitRepositoryResolver(apiBase, namespace, name string, token func() (string, error), client *http.Client) ArtifactResolver {
	endpoint := strings.TrimRight(apiBase, "/") + fmt.Sprintf(gitRepositoryPath, namespace, name)
	return func(ctx context.Context) (string, string, error) {
		tok, err := token()
		if err != nil {
			return "", "", fmt.Errorf("read service account token: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return "", "", fmt.Errorf("get gitrepository %s/%s: %w", namespace, name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("get gitrepository %s/%s: unexpected status code: %d", namespace, name, resp.StatusCode)
		}
		var gr struct {
			Status struct {
				Artifact struct {
					URL      string `json:"url"`
					Revision string `json:"revision"`
				} `json:"artifact"`
			} `json:"status"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&gr); err != nil {
			return "", "", fmt.Errorf("decode gitrepository: %w", err)
		}
		a := gr.Status.Artifact
		if a.URL == "" || a.Revision == "" {
			return "", "", fmt.Errorf("gitrepository %s/%s has no stored artifact yet", namespace, name)
		}
		return a.URL, a.Revision, nil
	}
}

// InClusterGitRepositoryResolver builds a resolver for "<namespace>/<name>"
// using the pod's service account and the in-cluster API server address.
func InClusterGitRepositoryResolver(ref string) (ArtifactResolver, error) {
	namespace, name, ok := strings.Cut(ref, "/")
	if !ok || namespace == "" || name == "" {
		return nil, fmt.Errorf("FLUX_GITREPOSITORY must be <namespace>/<name>, got %q", ref)
	}
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("not running in a cluster: KUBERNETES_SERVICE_HOST/PORT unset")
	}
	ca, err := os.ReadFile(serviceAccountDir + "/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("read cluster CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("cluster CA has no certificates")
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
	// Bound service account tokens rotate, so read the file on every call.
	token := func() (string, error) {
		b, err := os.ReadFile(serviceAccountDir + "/token")
		return strings.TrimSpace(string(b)), err
	}
	apiBase := "https://" + joinHostPort(host, port)
	return GitRepositoryResolver(apiBase, namespace, name, token, client), nil
}

func joinHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}
