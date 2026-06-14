package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func findVolume(volumes []corev1.Volume, name string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == name {
			return &volumes[i]
		}
	}
	return nil
}

func findMount(mounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == name {
			return &mounts[i]
		}
	}
	return nil
}

func TestBuildStatefulSetNoBrandingByDefault(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	sts := BuildStatefulSet(instance, nil)

	if v := findVolume(sts.Spec.Template.Spec.Volumes, BrandVolumeName); v != nil {
		t.Errorf("expected no brand volume when branding unset, got %+v", v)
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if m := findMount(container.VolumeMounts, BrandVolumeName); m != nil {
		t.Errorf("expected no brand volume mount when branding unset, got %+v", m)
	}
	for _, env := range container.Env {
		if env.Name == EnvBrandDir {
			t.Errorf("expected no %s env when branding unset", EnvBrandDir)
		}
	}
}

func TestBuildStatefulSetBrandingWiring(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Branding = &paperclipv1alpha1.BrandingSpec{
		CSSConfigMapRef: &corev1.LocalObjectReference{Name: "paperclip-brand-css"},
	}
	sts := BuildStatefulSet(instance, nil)

	vol := findVolume(sts.Spec.Template.Spec.Volumes, BrandVolumeName)
	if vol == nil {
		t.Fatalf("expected a %q volume", BrandVolumeName)
	}
	if vol.ConfigMap == nil {
		t.Fatalf("expected brand volume to be backed by a ConfigMap, got %+v", vol.VolumeSource)
	}
	if vol.ConfigMap.Name != "paperclip-brand-css" {
		t.Errorf("expected brand ConfigMap name paperclip-brand-css, got %q", vol.ConfigMap.Name)
	}

	container := sts.Spec.Template.Spec.Containers[0]
	mount := findMount(container.VolumeMounts, BrandVolumeName)
	if mount == nil {
		t.Fatalf("expected a %q volume mount on the main container", BrandVolumeName)
	}
	if mount.MountPath != BrandMountPath {
		t.Errorf("expected brand mount path %q, got %q", BrandMountPath, mount.MountPath)
	}
	if !mount.ReadOnly {
		t.Error("expected the brand mount to be read-only")
	}

	var brandDir string
	for _, env := range container.Env {
		if env.Name == EnvBrandDir {
			brandDir = env.Value
		}
	}
	if brandDir != BrandMountPath {
		t.Errorf("expected %s=%s, got %q", EnvBrandDir, BrandMountPath, brandDir)
	}
}
