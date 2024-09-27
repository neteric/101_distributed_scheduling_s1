package podapp

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := v.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Validating Pod(%s/%s) for request: %s", pod.Namespace, pod.Name, req.Operation)

	validatePodName := func(pod *corev1.Pod) bool {
		return len(pod.Name) > 5
	}
	if req.Operation == admissionv1.Update {
		oldPod := &corev1.Pod{}
		err = v.Decoder.DecodeRaw(req.OldObject, oldPod)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if validatePodName(pod) {
			klog.Errorf("Validating PodUpdate failed: pod name len > 5")
			return admission.Denied("pod name len > 5")
		}
	}
	// } else {
	// 	if errs := v.validateMCS(mcs); len(errs) != 0 {
	// 		klog.Errorf("Validating MultiClusterService failed: %v", errs)
	// 		return admission.Denied(errs.ToAggregate().Error())
	// 	}
	// }
	return admission.Allowed("")
}
