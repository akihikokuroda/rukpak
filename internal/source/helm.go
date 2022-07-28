package source

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/memfs"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/cli"
	corev1 "k8s.io/api/core/v1"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
)

type Helm struct {
	client.Reader
	SecretNamespace string
}

func (r *Helm) Unpack(ctx context.Context, bundle *rukpakv1alpha1.Bundle) (*Result, error) {
	if bundle.Spec.Source.Type != rukpakv1alpha1.SourceTypeHelm {
		return nil, fmt.Errorf("bundle source type %q not supported", bundle.Spec.Source.Type)
	}
	if bundle.Spec.Source.Helm == nil {
		return nil, fmt.Errorf("bundle source heml configuration is unset")
	}
	helmsource := bundle.Spec.Source.Helm
	if helmsource.Repository == "" {
		// This should never happen because the validation webhook rejects helm bundles without repository
		return nil, errors.New("missing helm source information: repository must be provided")
	}
	var out strings.Builder
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(true),
	)
	if err != nil {
		return nil, err
	}
	
	var options []getter.Option
	options = append(options, getter.WithInsecureSkipVerifyTLS(bundle.Spec.Source.Helm.Auth.InsecureSkipVerify))
	var userName, password string
	if bundle.Spec.Source.Helm.Auth.Secret.Name != "" {
		userName, password, err = r.getCredentials(ctx, bundle)
		if err != nil {
			return nil, err
		}
		options = append(options, getter.WithBasicAuth(userName, password))
	}
	registryClient.Login(helmsource.Repository,
		registry.LoginOptBasicAuth(userName, password),
		registry.LoginOptInsecure(bundle.Spec.Source.Helm.Auth.InsecureSkipVerify),
		)
	c := downloader.ChartDownloader{
		Out:     &out,
		Getters: getter.All(&cli.EnvSettings{}),
		RegistryClient:   registryClient,
		Options:  options,
		
	}
	fmt.Printf("CHART: %s %s %s\n", helmsource.Repository, helmsource.ChartName, helmsource.ChartVersion)
	if _, err = getter.All(&cli.EnvSettings{}).ByScheme("oci"); err != nil {
		fmt.Printf("ERROR No GETTER\n")
	}
	chartURL, err := repo.FindChartInAuthAndTLSRepoURL(helmsource.Repository, userName, password, helmsource.ChartName, helmsource.ChartVersion, "","","", bundle.Spec.Source.Helm.Auth.InsecureSkipVerify, getter.All(&cli.EnvSettings{}))
	if err != nil {
//		return nil, err
		chartURL = "oci://docker-registry.rukpak-e2e.svc.cluster.local:5000/helm-charts/mychart-0.1.0.tgz"
	}
	fmt.Printf("chartURL: %s\n", chartURL)
	saved, _, err := c.DownloadTo(chartURL, helmsource.ChartVersion, "/")
	if err != nil {
		return nil, err
	}
	
	var memFS = memfs.New()
	file, err := memFS.Create(filepath.Join("manifests", "chart"))
	if err != nil {
		return nil, fmt.Errorf("creating filesystem: %s", err)
	}
	chart, err := ioutil.ReadFile(saved)
	if err != nil {
		return nil, fmt.Errorf("reading downloaded chart file: %s", err)
	}
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
		Type:  rukpakv1alpha1.SourceTypeHelm,
		Helm: bundle.Spec.Source.Helm.DeepCopy(),
	}

	return &Result{Bundle: bundleFS, ResolvedSource: resolvedSource, State: StateUnpacked}, nil
}

// getCredentials reads credentials from the secret specified in the bundle
// It returns the username ane password when they are in the secret
func (r *Helm) getCredentials(ctx context.Context, bundle *rukpakv1alpha1.Bundle) (string, string, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: r.SecretNamespace, Name: bundle.Spec.Source.Helm.Auth.Secret.Name}, secret)
	if err != nil {
		return "", "", err
	}
	userName := string(secret.Data["username"])
	password := string(secret.Data["password"])

	return userName, password, nil
}

