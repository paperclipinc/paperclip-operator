package resources

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

const (
	// DefaultTailscaleImage is the default image repository for the Tailscale sidecar.
	DefaultTailscaleImage = "ghcr.io/tailscale/tailscale"
	// DefaultTailscaleTag is the default Tailscale sidecar image tag.
	DefaultTailscaleTag = "stable"
	// DefaultTailscaleAuthKeyKey is the default Secret key holding the auth key.
	DefaultTailscaleAuthKeyKey = "authkey"

	// TailscaleContainerName is the name of the Tailscale sidecar container.
	TailscaleContainerName = "tailscale"

	// TailscaleModeServe exposes the instance to tailnet members only.
	TailscaleModeServe = "serve"
	// TailscaleModeFunnel exposes the instance to the public internet via Funnel.
	TailscaleModeFunnel = "funnel"

	// TailscaleStatePath is the directory where tailscaled stores state. Placed
	// under an emptyDir so tailscaled owns the path with a read-only root fs.
	TailscaleStatePath = "/tmp/tailscale"
	// TailscaleSocketDir is the directory containing the tailscaled Unix socket.
	TailscaleSocketDir = "/var/run/tailscale"
	// TailscaleSocketPath is the full path to the tailscaled Unix socket.
	TailscaleSocketPath = "/var/run/tailscale/tailscaled.sock"

	// TailscaleServeConfigKey is the ConfigMap data key for the serve config JSON.
	TailscaleServeConfigKey = "tailscale-serve.json"
	// tailscaleSocketVolumeName is the emptyDir volume holding the tailscaled socket.
	tailscaleSocketVolumeName = "tailscale-socket"
	// tailscaleStateVolumeName is the emptyDir volume holding tailscaled state.
	tailscaleStateVolumeName = "tailscale-tmp"
	// tailscaleConfigVolumeName is the ConfigMap volume holding the serve config.
	tailscaleConfigVolumeName = "tailscale-config"
)

// TailscaleConfigMapName returns the name of the Tailscale serve-config ConfigMap.
func TailscaleConfigMapName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-tailscale"
}

// tailscaleServeConfig is the JSON structure for TS_SERVE_CONFIG.
type tailscaleServeConfig struct {
	TCP map[string]*tailscaleTCPHandler `json:"TCP"`
	Web map[string]*tailscaleWebConfig  `json:"Web,omitempty"`
	// AllowFunnel controls whether Tailscale Funnel (public internet) is enabled.
	AllowFunnel map[string]bool `json:"AllowFunnel,omitempty"`
}

type tailscaleTCPHandler struct {
	HTTPS bool `json:"HTTPS"`
}

type tailscaleWebConfig struct {
	Handlers map[string]*tailscaleWebHandler `json:"Handlers"`
}

type tailscaleWebHandler struct {
	Proxy string `json:"Proxy"`
}

// GetTailscaleImage returns the full Tailscale sidecar image reference.
func GetTailscaleImage(instance *paperclipv1alpha1.Instance) string {
	repo := instance.Spec.Tailscale.Image.Repository
	if repo == "" {
		repo = DefaultTailscaleImage
	}
	if instance.Spec.Tailscale.Image.Digest != "" {
		return repo + "@" + instance.Spec.Tailscale.Image.Digest
	}
	tag := instance.Spec.Tailscale.Image.Tag
	if tag == "" {
		tag = DefaultTailscaleTag
	}
	return repo + ":" + tag
}

// TailscaleHostname returns the configured Tailscale device name, defaulting to
// the instance name.
func TailscaleHostname(instance *paperclipv1alpha1.Instance) string {
	if instance.Spec.Tailscale.Hostname != "" {
		return instance.Spec.Tailscale.Hostname
	}
	return instance.Name
}

// BuildTailscaleServeConfig returns the TS_SERVE_CONFIG JSON that proxies the
// tailnet HTTPS endpoint to the local Paperclip app port. Funnel is enabled
// when mode is "funnel".
func BuildTailscaleServeConfig(instance *paperclipv1alpha1.Instance) string {
	proxy := fmt.Sprintf("http://127.0.0.1:%d", servicePort(instance))

	cfg := tailscaleServeConfig{
		TCP: map[string]*tailscaleTCPHandler{
			"443": {HTTPS: true},
		},
		Web: map[string]*tailscaleWebConfig{
			"${TS_CERT_DOMAIN}:443": {
				Handlers: map[string]*tailscaleWebHandler{
					"/": {Proxy: proxy},
				},
			},
		},
	}

	mode := instance.Spec.Tailscale.Mode
	if mode == "" {
		mode = TailscaleModeServe
	}
	if mode == TailscaleModeFunnel {
		cfg.AllowFunnel = map[string]bool{
			"${TS_CERT_DOMAIN}:443": true,
		}
	}

	data, _ := json.Marshal(cfg)
	return string(data)
}

// BuildTailscaleContainer builds the ephemeral Tailscale sidecar container that
// runs userspace tailscaled and Serves the Paperclip app over the tailnet.
func BuildTailscaleContainer(instance *paperclipv1alpha1.Instance) corev1.Container {
	env := []corev1.EnvVar{
		{Name: "TS_USERSPACE", Value: "true"},
		{Name: "TS_STATE_DIR", Value: TailscaleStatePath},
		{Name: "TS_SOCKET", Value: TailscaleSocketPath},
		{Name: "TS_SERVE_CONFIG", Value: "/etc/tailscale/serve/" + TailscaleServeConfigKey},
		{Name: "TS_HOSTNAME", Value: TailscaleHostname(instance)},
		// Ephemeral nodes are removed from the tailnet when the pod is deleted.
		{Name: "TS_EXTRA_ARGS", Value: "--ephemeral"},
	}

	if instance.Spec.Tailscale.AuthKey != nil {
		key := instance.Spec.Tailscale.AuthKey.Key
		if key == "" {
			key = DefaultTailscaleAuthKeyKey
		}
		env = append(env, corev1.EnvVar{
			Name: "TS_AUTHKEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: instance.Spec.Tailscale.AuthKey.SecretRef,
					Key:                  key,
				},
			},
		})
	}

	return corev1.Container{
		Name:            TailscaleContainerName,
		Image:           GetTailscaleImage(instance),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             env,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      tailscaleSocketVolumeName,
				MountPath: TailscaleSocketDir,
			},
			{
				Name:      tailscaleConfigVolumeName,
				MountPath: "/etc/tailscale/serve/" + TailscaleServeConfigKey,
				SubPath:   TailscaleServeConfigKey,
				ReadOnly:  true,
			},
			{
				Name:      tailscaleStateVolumeName,
				MountPath: "/tmp",
			},
		},
		Resources: instance.Spec.Tailscale.Resources,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			RunAsNonRoot:             Ptr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	}
}

// TailscaleVolumes returns the volumes required by the Tailscale sidecar: two
// emptyDirs (socket dir and state dir) and the serve-config ConfigMap.
func TailscaleVolumes(instance *paperclipv1alpha1.Instance) []corev1.Volume {
	return []corev1.Volume{
		{
			Name:         tailscaleSocketVolumeName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         tailscaleStateVolumeName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name: tailscaleConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: TailscaleConfigMapName(instance),
					},
				},
			},
		},
	}
}

// BuildTailscaleConfigMap builds the ConfigMap holding the TS_SERVE_CONFIG JSON
// mounted into the Tailscale sidecar.
func BuildTailscaleConfigMap(instance *paperclipv1alpha1.Instance) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: ObjectMeta(instance, TailscaleConfigMapName(instance)),
		Data: map[string]string{
			TailscaleServeConfigKey: BuildTailscaleServeConfig(instance),
		},
	}
}
