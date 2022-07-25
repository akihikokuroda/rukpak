package source

import (
//	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/memfs"
//	"github.com/go-git/go-git/v5"
//	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
//	"github.com/go-git/go-git/v5/storage/memory"
//	"golang.org/x/crypto/ssh"
//	sshgit "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
//	"github.com/go-git/go-billy/v5/memfs"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/cli"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
)

type HTTP struct {
	client.Reader
	SecretNamespace string
}

func (r *HTTP) Unpack(ctx context.Context, bundle *rukpakv1alpha1.Bundle) (*Result, error) {
	if bundle.Spec.Source.Type != rukpakv1alpha1.SourceTypeHTTP {
		return nil, fmt.Errorf("bundle source type %q not supported", bundle.Spec.Source.Type)
	}
	if bundle.Spec.Source.HTTP == nil {
		return nil, fmt.Errorf("bundle source http configuration is unset")
	}
	httpsource := bundle.Spec.Source.HTTP
	if httpsource.Repository == "" {
		// This should never happen because the validation webhook rejects http bundles without repository
		return nil, errors.New("missing http source information: repository must be provided")
	}
	var out strings.Builder
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(false),
//		registry.ClientOptEnableCache(true),
//		registry.ClientOptWriter(out),
	)
	if err != nil {
		return nil, err
	}
	c := downloader.ChartDownloader{
		Out:     &out,
		Getters: getter.All(&cli.EnvSettings{}),
		RegistryClient:   registryClient,
		
	}
	chartURL, err := repo.FindChartInRepoURL(httpsource.Repository, "apache", "9.1.13", "","","",  getter.All(&cli.EnvSettings{}) )
	if err != nil {
		return nil, err
	}
	fmt.Printf("\n\nDOWNLOADED 1: %v\n\n", chartURL)
	saved, _, err := c.DownloadTo(chartURL, "9.1.13", "/")
	if err != nil {
	fmt.Printf("DOWNLOADED 1.1: %+v\n\n", err)
		return nil, err
	}
	
	fmt.Printf("\n\nDOWNLOADED 2: %v\n\n", saved)
	var memFS = memfs.New()
	file, err := memFS.Create(filepath.Join("manifests", "chart"))
	if err != nil {
		return nil, fmt.Errorf("creating filesystem: %s", err)
	}
	chart, err := ioutil.ReadFile(saved)
	if err != nil {
		return nil, fmt.Errorf("reading downloaded chart file: %s", err)
	}
//	fmt.Printf("\n\nDOWNLOADED 3: %s\n\n", string(chart))
	_, err = file.Write(chart)
	if err != nil {
		return nil, fmt.Errorf("writing chart file: %s", err)
	}
	err = file.Close()
	if err != nil {
		return nil, fmt.Errorf("closing chart file: %s", err)
	}
	var bundleFS fs.FS = &billyFS{memFS}
	resolvedSource := &rukpakv1alpha1.BundleSource{
		Type:  rukpakv1alpha1.SourceTypeHTTP,
		HTTP: bundle.Spec.Source.HTTP.DeepCopy(),
	}

	return &Result{Bundle: bundleFS, ResolvedSource: resolvedSource, State: StateUnpacked}, nil
}

func (r *HTTP) configAuth(ctx context.Context, bundle *rukpakv1alpha1.Bundle) (transport.AuthMethod, error) {
	var auth transport.AuthMethod
	if strings.HasPrefix(bundle.Spec.Source.HTTP.Repository, "http") {
		userName, password, err := r.getCredentials(ctx, bundle)
		if err != nil {
			return nil, err
		}
		return &http.BasicAuth{Username: userName, Password: password}, nil
	}
	return auth, nil
}

// getCredentials reads credentials from the secret specified in the bundle
// It returns the username ane password when they are in the secret
func (r *HTTP) getCredentials(ctx context.Context, bundle *rukpakv1alpha1.Bundle) (string, string, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: r.SecretNamespace, Name: bundle.Spec.Source.Git.Auth.Secret.Name}, secret)
	if err != nil {
		return "", "", err
	}
	userName := string(secret.Data["username"])
	password := string(secret.Data["password"])

	return userName, password, nil
}

