#! /bin/bash

WEBHOOK_NS=k8s-webhook-template
WEBHOOK_NAME=k8s-webhook-template
WEBHOOK_SVC=${WEBHOOK_NAME}.${WEBHOOK_NS}.svc
K8S_OUT_CERT_FILE=./deploy/certs.yaml

OUT_CERT="./webhookCA.crt"
OUT_KEY="./webhookCA.key"
   
# Create certs for our webhook 
set -f 
mkcert \
  --cert-file "${OUT_CERT}" \
  --key-file "${OUT_KEY}" \
  "${WEBHOOK_SVC}" 192.168.254.2
set +f

# Create certs secrets for k8s.
rm ${K8S_OUT_CERT_FILE}
kubectl -n ${WEBHOOK_NS} create secret generic \
    ${WEBHOOK_NAME}-certs \
    --from-file=key.pem=${OUT_KEY} \
    --from-file=cert.pem=${OUT_CERT}\
    --dry-run=client -o yaml > ${K8S_OUT_CERT_FILE}
