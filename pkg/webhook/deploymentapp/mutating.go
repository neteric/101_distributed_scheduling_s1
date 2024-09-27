package deploymentapp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	coorv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
	Client  kubernetes.Clientset
}

const (
	AnnotationLowWaterLevel        string = "webhook-demo.com/low-water-level"
	AnnotationHighWaterLevel       string = "webhook-demo.com/high-water-level"
	AnnotationScheduleCompensation string = "webhook-demo.com/schedule-compensation"
	OnDemandNodeLabelKey           string = "node.kubernetes.io/capacity"
	OnDemandNodeLabelValue         string = "on-demand"
	SpotNodeLabelKey               string = "node.kubernetes.io/capacity"
	SpotNodeLabelValue             string = "spot"
)

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)

	strategy, err := a.GetAnnotationsOfDeployment(ctx, pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !a.shouldMutate(strategy) {
		klog.V(2).Infof("Skip mutating Pod(%s/%s) for request: %s", req.Namespace, pod.Name, req.Operation)
		return admission.Allowed("")
	}

	if !a.tryAcquireLock(ctx, pod) {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	defer a.releaseLock(ctx, pod)

	// list all pod have same label
	podList, err := a.Client.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(pod.Labels).String(),
	})
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if a.countOnDemandPod(podList) <= strategy.LowWaterLevel {
		a.ensureOnDemandNodeAffinityOfPod(pod)
	} else {
		a.ensureSpotNodeAffinityOfPod(pod)
	}

	marshaledBytes, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (a *MutatingAdmission) shouldMutate(s *UserStrategy) bool {
	return s.ScheduleCompensation != nil && *s.ScheduleCompensation &&
		s.LowWaterLevel > 0 && s.HighWaterLevel > 0
}

func (a *MutatingAdmission) ensureOnDemandNodeAffinityOfPod(pod *corev1.Pod) {
	OnDemandNodeAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      OnDemandNodeLabelKey,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{OnDemandNodeLabelValue},
							},
						},
					},
				},
			},
		},
	}
	a.mergeNodeAffinity("on-demand", pod, OnDemandNodeAffinity)
}

func (a *MutatingAdmission) ensureSpotNodeAffinityOfPod(pod *corev1.Pod) {
	SpotNodeAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 100,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      SpotNodeLabelKey,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{SpotNodeLabelValue},
							},
						},
					},
				},
			},
		},
	}
	a.mergeNodeAffinity("spot", pod, SpotNodeAffinity)
}

func (a *MutatingAdmission) countOnDemandPod(podList *corev1.PodList) int {
	count := 0
	for _, pod := range podList.Items {
		if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil || pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil || pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
			continue
		}
		for _, k := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			if k.MatchExpressions == nil {
				continue
			}
			for _, v := range k.MatchExpressions {
				if v.Key == OnDemandNodeLabelKey && v.Operator == corev1.NodeSelectorOpIn && v.Values[0] == OnDemandNodeLabelValue {
					count++
				}
			}
		}
	}
	return count
}

func (a *MutatingAdmission) mergeNodeAffinity(t string, pod *corev1.Pod, affinity *corev1.Affinity) {
	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = affinity
	} else if pod.Spec.Affinity.NodeAffinity == nil {
		pod.Spec.Affinity.NodeAffinity = affinity.NodeAffinity

	} else {
		switch t {
		case "on-demand":
			if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			} else if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
			} else {
				pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms...)
			}
		case "spot":
			if pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
				pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			} else {
				pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
			}
		}

	}
}

func (a *MutatingAdmission) tryAcquireLock(ctx context.Context, pod *corev1.Pod) bool {
	lease := &coorv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.GenerateName + "-" + "lease",
			Namespace: pod.Namespace,
		},
		Spec: coorv1.LeaseSpec{
			HolderIdentity: &pod.Name,
		},
	}
	ticker := time.NewTicker(time.Second * 3)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			_, err := a.Client.CoordinationV1().Leases(pod.Namespace).Create(ctx, lease, metav1.CreateOptions{})
			if err == nil {
				return true
			}
		}

	}
}

func (a *MutatingAdmission) releaseLock(ctx context.Context, pod *corev1.Pod) error {
	leaseName := pod.GenerateName + "-" + "lease"
	return a.Client.CoordinationV1().Leases(pod.Namespace).Delete(ctx, leaseName, metav1.DeleteOptions{})
}

type UserStrategy struct {
	LowWaterLevel        int
	HighWaterLevel       int
	ScheduleCompensation *bool
}

func (a *MutatingAdmission) GetAnnotationsOfDeployment(ctx context.Context, pod *corev1.Pod) (*UserStrategy, error) {
	var replisetName string
	for _, v := range pod.OwnerReferences {
		if *v.Controller {
			replisetName = v.Name
			break
		}
	}
	repliset, err := a.Client.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, replisetName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var deployName string
	for _, v := range repliset.OwnerReferences {
		if *v.Controller {
			deployName = v.Name
			break
		}
	}
	deploy, err := a.Client.AppsV1().Deployments(pod.Namespace).Get(ctx, deployName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &UserStrategy{
		LowWaterLevel:        GetWaterLevel(deploy.GetAnnotations(), AnnotationLowWaterLevel),
		HighWaterLevel:       GetWaterLevel(deploy.GetAnnotations(), AnnotationHighWaterLevel),
		ScheduleCompensation: ScheduleCompensation(deploy.GetAnnotations(), AnnotationScheduleCompensation),
	}, nil
}
func GetWaterLevel(annotations map[string]string, key string) int {
	if value, ok := annotations[key]; ok {
		v, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}

func ScheduleCompensation(annotations map[string]string, key string) *bool {
	if value, ok := annotations[key]; ok {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return nil
		}
		return &v
	}
	return nil
}
