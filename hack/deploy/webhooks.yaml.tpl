apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: k8s-webhook-template-webhook
  labels:
    app: k8s-webhook-template-webhook
    kind: mutator
webhooks:
  - name: all-mark-webhook.slok.dev
    # Avoid chicken-egg problem with our webhook deployment.
    objectSelector:
      matchExpressions:
      - key: app
        operator: NotIn
        values: ["k8s-webhook-template"]
    admissionReviewVersions: ["v1"]
    sideEffects: None
    clientConfig:
      service:
        name: k8s-webhook-template
        namespace: k8s-webhook-template
        path: /wh/mutating/allmark
      caBundle: CA_BUNDLE
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["*"]
        apiVersions: ["*"]
        resources: ["deployments", "daemonsets", "cronjobs", "jobs", "statefulsets", "pods"]

  - name: service-monitor-safer.slok.dev
    admissionReviewVersions: ["v1"]
    sideEffects: None
    clientConfig:
      service:
        name: k8s-webhook-template
        namespace: k8s-webhook-template
        path: /wh/mutating/safeservicemonitor
      caBundle: CA_BUNDLE
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["monitoring.coreos.com"]
        apiVersions: ["v1"]
        resources: ["servicemonitors"]

---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: k8s-webhook-template-webhook
  labels:
    app: k8s-webhook-template-webhook
    kind: validator
webhooks:
  - name: ingress-validation-webhook.slok.dev
    admissionReviewVersions: ["v1"]
    sideEffects: None
    clientConfig:
      service:
        name: k8s-webhook-template
        namespace: k8s-webhook-template
        path: /wh/validating/ingress
      caBundle: CA_BUNDLE
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["*"]
        apiVersions: ["*"]
        resources: ["ingresses"]