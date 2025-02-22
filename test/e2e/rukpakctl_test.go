package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	plain "github.com/operator-framework/rukpak/internal/provisioner/plain/types"
)

const (
	rukpakctlcmd = "../../bin/rukpakctl "
	testbundles  = "../../testdata/bundles/"
)

var _ = Describe("rukpakctl run subcommand", func() {
	When("run executed with a valid local manifest directory", func() {
		var (
			ctx                  context.Context
			bundlename           string
			bundle               *rukpakv1alpha1.Bundle
			bundledeploymentname string
			bundledeployment     *rukpakv1alpha1.BundleDeployment
		)
		BeforeEach(func() {
			ctx = context.Background()
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"run test "+testbundles+"plain-v0/valid").Output()
			Expect(err).To(BeNil())
			fmt.Sscanf(string(out), "bundledeployment.core.rukpak.io %q applied\nsuccessfully uploaded bundle content for %q", &bundledeploymentname, &bundlename)
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, bundledeployment)).To(BeNil())
		})
		It("should eventually report a successful state", func() {
			bundle = &rukpakv1alpha1.Bundle{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundlename,
				},
			}
			By("eventually reporting an Unpacked phase", func() {
				Eventually(func() (string, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return "", err
					}
					return bundle.Status.Phase, nil
				}).Should(Equal(rukpakv1alpha1.PhaseUnpacked))
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundle.Status.Conditions, rukpakv1alpha1.TypeUnpacked), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeUnpacked)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonUnpackSuccessful)),
				))
			})
			bundledeployment = &rukpakv1alpha1.BundleDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundledeploymentname,
				},
			}
			By("eventually reporting Unpacked", func() {
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundledeployment), bundledeployment); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundledeployment.Status.Conditions, rukpakv1alpha1.TypeHasValidBundle), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeHasValidBundle)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonUnpackSuccessful)),
				))
			})
			By("eventually reporting Installed", func() {
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundledeployment), bundledeployment); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundledeployment.Status.Conditions, rukpakv1alpha1.TypeInstalled), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeInstalled)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonInstallationSucceeded)),
				))
			})
		})
	})
	When("run executed with an invalid manifest directory", func() {
		var (
			message string
		)
		BeforeEach(func() {
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"run test "+testbundles+"plain-v0/notvalid").CombinedOutput()
			message = string(out)
			Expect(err).NotTo(BeNil())
		})
		It(`should fail with a "no such file or directory" message`, func() {
			Expect(strings.Contains(message, "no such file or directory")).To(BeTrue())
		})
	})
	When("run executed with a bundle cannot unpacked", func() {
		var (
			ctx                  context.Context
			bundlename           string
			bundle               *rukpakv1alpha1.Bundle
			bundledeploymentname string
			bundledeployment     *rukpakv1alpha1.BundleDeployment
		)
		BeforeEach(func() {
			ctx = context.Background()
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"run test "+testbundles+"plain-v0/subdir").Output()
			Expect(err).To(BeNil())
			fmt.Sscanf(string(out), "bundledeployment.core.rukpak.io %q applied\nsuccessfully uploaded bundle content for %q", &bundledeploymentname, &bundlename)
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, bundledeployment)).To(BeNil())
		})
		It("should eventually report unpack fail", func() {
			bundle = &rukpakv1alpha1.Bundle{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundlename,
				},
			}
			By("eventually reporting an Unpacked phase", func() {
				Eventually(func() (string, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return "", err
					}
					return bundle.Status.Phase, nil
				}).Should(Equal(rukpakv1alpha1.PhaseFailing))
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundle.Status.Conditions, rukpakv1alpha1.TypeUnpacked), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeUnpacked)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionFalse)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonUnpackFailed)),
				))
			})
			bundledeployment = &rukpakv1alpha1.BundleDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundledeploymentname,
				},
			}
			By("eventually reporting Unpack failed", func() {
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundledeployment), bundledeployment); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundledeployment.Status.Conditions, rukpakv1alpha1.TypeHasValidBundle), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeHasValidBundle)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionFalse)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonUnpackFailed)),
				))
			})
		})
	})
})

var _ = Describe("rukpakctl content subcommand", func() {
	When("content executed with a valid bundle", func() {
		var (
			ctx    context.Context
			bundle *rukpakv1alpha1.Bundle
			output string
		)
		BeforeEach(func() {
			ctx = context.Background()
			bundle = &rukpakv1alpha1.Bundle{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "combo-git-commit",
				},
				Spec: rukpakv1alpha1.BundleSpec{
					ProvisionerClassName: plain.ProvisionerID,
					Source: rukpakv1alpha1.BundleSource{
						Type: rukpakv1alpha1.SourceTypeGit,
						Git: &rukpakv1alpha1.GitSource{
							Repository: "https://github.com/exdx/combo-bundle",
							Ref: rukpakv1alpha1.GitRef{
								Commit: "9e3ab7f1a36302ef512294d5c9f2e9b9566b811e",
							},
						},
					},
				},
			}
			err := c.Create(ctx, bundle)
			Expect(err).To(BeNil())
			By("eventually reporting an Unpacked phase", func() {
				Eventually(func() (string, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return "", err
					}
					return bundle.Status.Phase, nil
				}).Should(Equal(rukpakv1alpha1.PhaseUnpacked))
			})
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"content "+bundle.ObjectMeta.Name).Output()
			Expect(err).To(BeNil())
			output = string(out)
		})
		AfterEach(func() {
			err := c.Delete(ctx, bundle)
			Expect(err).To(BeNil())
		})

		It("output all files in the bundle", func() {
			Expect(strings.Contains(output, "manifests/00_namespace.yaml") &&
				strings.Contains(output, "manifests/01_cluster_role.yaml") &&
				strings.Contains(output, "manifests/01_service_account.yaml") &&
				strings.Contains(output, "manifests/02_deployment.yaml") &&
				strings.Contains(output, "manifests/03_cluster_role_binding.yaml") &&
				strings.Contains(output, "manifests/combo.io_combinations.yaml") &&
				strings.Contains(output, "manifests/combo.io_templates.yaml")).To(BeTrue())
		})
	})
	When("content executed with a wrong bundle name", func() {
		var (
			output string
		)
		BeforeEach(func() {
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"content badname").Output()
			Expect(err).NotTo(BeNil())
			output = string(out)
		})

		It("output error message", func() {
			Expect(strings.Contains(output, "content command failed") &&
				strings.Contains(output, "bundles.core.rukpak.io \"a\" not found"))
		})
	})
	When("content executed on a failed bundle", func() {
		var (
			ctx                  context.Context
			bundlename           string
			bundle               *rukpakv1alpha1.Bundle
			bundledeploymentname string
			bundledeployment     *rukpakv1alpha1.BundleDeployment
			output               string
		)
		BeforeEach(func() {
			ctx = context.Background()
			out, err := exec.Command("sh", "-c", rukpakctlcmd+"run test "+testbundles+"plain-v0/subdir").Output()
			Expect(err).To(BeNil())
			fmt.Sscanf(string(out), "bundledeployment.core.rukpak.io %q applied\nsuccessfully uploaded bundle content for %q", &bundledeploymentname, &bundlename)
			bundledeployment = &rukpakv1alpha1.BundleDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundledeploymentname,
				},
			}
			bundle = &rukpakv1alpha1.Bundle{
				ObjectMeta: metav1.ObjectMeta{
					Name: bundlename,
				},
			}
			By("eventually reporting an Unpacked phase", func() {
				Eventually(func() (string, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return "", err
					}
					return bundle.Status.Phase, nil
				}).Should(Equal(rukpakv1alpha1.PhaseFailing))
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(bundle), bundle); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(bundle.Status.Conditions, rukpakv1alpha1.TypeUnpacked), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeUnpacked)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionFalse)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonUnpackFailed)),
				))
				out, err := exec.Command("sh", "-c", rukpakctlcmd+"content "+bundlename).CombinedOutput()
				Expect(err).NotTo(BeNil())
				output = string(out)
			})
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, bundledeployment)).To(BeNil())
		})
		It("should eventually report a failure", func() {
			Expect(strings.Contains(output, "content command failed: error: url is not available")).To(BeTrue())
		})
	})

})
