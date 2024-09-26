package pod

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type fakeMutationDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeMutationDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeMutationDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

func TestMutatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    admission.Response
	}{
		{
			name: "Handle_DecodeError_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req:  admission.Request{},
			want: admission.Errored(http.StatusBadRequest, errors.New("decode error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MutatingAdmission{
				Decoder: tt.decoder,
			}
			got := m.Handle(context.Background(), tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutatingAdmission_Handle_FullCoverage(t *testing.T) {
	name := "test-pod"
	namespace := "test-namespace"

	// Mock a request with a specific namespace.
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Name:      name,
			Namespace: namespace,
		},
	}

	// Create the initial mcs with default values for testing.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			ResourceVersion:   "1001",
			CreationTimestamp: metav1.Time{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
		Status: corev1.PodStatus{},
	}
	// "{\"metadata\":{\"name\":\"test-pod\",\"namespace\":\"test-namespace\",\"resourceVersion\":\"1001\",\"creationTimestamp\":null,\"labels\":{\"ID\":\"some-unique-id\"}},\"spec\":{\"containers\":null},\"status\":{}}"
	// Define the expected mcs object after mutations.
	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: "1001",
			Labels: map[string]string{
				"ID": "some-unique-id",
			},
			CreationTimestamp: metav1.Time{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
		Status: corev1.PodStatus{},
	}

	// Mock decoder that decodes the request into the mcs object.
	decoder := &fakeMutationDecoder{
		obj: pod,
	}

	// Marshal the expected policy to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantPod)
	if err != nil {
		t.Fatalf("Failed to marshal expected policy: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := MutatingAdmission{
		Decoder: decoder,
	}

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Verify that the only patch applied is for the UUID label. If any other patches are present, it indicates that the mcs object was not handled as expected.
	if len(got.Patches) > 0 {
		firstPatch := got.Patches[0]
		if firstPatch.Operation != "replace" || firstPatch.Path != "/metadata/labels/ID" {
			t.Errorf("Handle() returned unexpected patches. Only the UUID patch was expected. Received patches: %v", got.Patches)
		}
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
