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

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
)

type Helm struct {
	client.Reader
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
		registry.ClientOptDebug(false),
	)
	if err != nil {
		return nil, err
	}
	c := downloader.ChartDownloader{
		Out:     &out,
		Getters: getter.All(&cli.EnvSettings{}),
		RegistryClient:   registryClient,
		
	}
	chartURL, err := repo.FindChartInRepoURL(helmsource.Repository, helmsource.ChartName, helmsource.ChartVersion, "","","",  getter.All(&cli.EnvSettings{}) )
	if err != nil {
		return nil, err
	}
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


