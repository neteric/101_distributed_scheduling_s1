package pod

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)

	// MutatingAdmission handle MUST idempotent
	DedupeAndMergeLabels := func(existLabel, newLabel map[string]string) map[string]string {
		if existLabel == nil {
			return newLabel
		}

		for k, v := range newLabel {
			existLabel[k] = v
		}
		return existLabel
	}

	// GetLabelValue retrieves the value via 'labelKey' if exist, otherwise returns an empty string.
	GetLabelValue := func(labels map[string]string, labelKey string) string {
		if labels == nil {
			return ""
		}

		return labels[labelKey]
	}

	if GetLabelValue(pod.Labels, "ID") == "" {
		id := uuid.New().String()
		pod.Labels = DedupeAndMergeLabels(pod.Labels, map[string]string{"ID": id})
	}

	marshaledBytes, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
