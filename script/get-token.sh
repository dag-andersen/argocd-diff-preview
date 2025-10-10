# ArgoCD credentials
ARGOCD_SERVER=https://localhost:8080
ARGOCD_PASSWD=$(kubectl -n argocd-diff-preview get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
echo "ARGOCD_PASSWD: $ARGOCD_PASSWD"
ARGOCD_TOKEN=$(curl -ks -H "Content-Type: application/json" "$ARGOCD_SERVER/api/v1/session" -d '{"username":"admin","password":"'"$ARGOCD_PASSWD"'"}' | jq -r '.token')
echo "ARGOCD_TOKEN: $ARGOCD_TOKEN"
kubectl get applications -n argocd-diff-preview

# retrieve resources via API call
# would work as well: https://localhost:8080/api/v1/applications/{applicationName}/managed-resources
echo "curl -ks -H \"Authorization: Bearer $ARGOCD_TOKEN\" \"$ARGOCD_SERVER/api/v1/applications/8368c-t-nginx-ingress/manifests\" | jq"
