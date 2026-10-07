{{- /*
Helpers the templates share, in the shape `helm create` gives them. Every per-component helper takes a dict holding
`root`, the chart's top-level context, and `component`, the component's directory word, which is also the key of its
section in the values.
*/ -}}

{{- /* The chart's name, which nameOverride replaces. */ -}}
{{- define "mediated-mailbox.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- /*
The release's fully qualified name, which fullnameOverride replaces. A release whose name already holds the chart's
name is used as it is.
*/ -}}
{{- define "mediated-mailbox.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- /* The chart label's value, its name and version. */ -}}
{{- define "mediated-mailbox.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- /* One component's name, the fullname followed by the component's word. */ -}}
{{- define "mediated-mailbox.componentName" -}}
{{- printf "%s-%s" (include "mediated-mailbox.fullname" .root) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- /* The labels that select one component's pods. */ -}}
{{- define "mediated-mailbox.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mediated-mailbox.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- /* Every label one component's objects carry. */ -}}
{{- define "mediated-mailbox.labels" -}}
helm.sh/chart: {{ include "mediated-mailbox.chart" .root }}
{{ include "mediated-mailbox.selectorLabels" . }}
app.kubernetes.io/part-of: {{ include "mediated-mailbox.name" .root }}
{{- if .root.Chart.AppVersion }}
app.kubernetes.io/version: {{ .root.Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end }}

{{- /* One component's values. */ -}}
{{- define "mediated-mailbox.values" -}}
{{- toYaml (index .root.Values .component) }}
{{- end }}

{{- /*
One component's image. Every image moves in lockstep at the release's version, which the release workflow sets as the
chart's appVersion (ADR-0049), so a tag is set only to deploy another version.
*/ -}}
{{- define "mediated-mailbox.image" -}}
{{- $image := (index .root.Values .component).image -}}
{{- printf "%s:%s" $image.repository ($image.tag | default .root.Chart.AppVersion) -}}
{{- end }}

{{- /* The name of the service account one component's pods run as. */ -}}
{{- define "mediated-mailbox.serviceAccountName" -}}
{{- $sa := (index .root.Values .component).serviceAccount -}}
{{- if $sa.create }}
{{- default (include "mediated-mailbox.componentName" .) $sa.name }}
{{- else }}
{{- default "default" $sa.name }}
{{- end }}
{{- end }}

{{- /* One component's service account, when the chart creates it. */ -}}
{{- define "mediated-mailbox.serviceAccount" -}}
{{- $sa := (index .root.Values .component).serviceAccount -}}
{{- if $sa.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "mediated-mailbox.serviceAccountName" . }}
  labels:
    {{- include "mediated-mailbox.labels" . | nindent 4 }}
  {{- with $sa.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: {{ $sa.automount }}
{{- end }}
{{- end }}

{{- /*
The pod template's metadata: the labels, the pod annotations and pod labels the values add, and the checksum of the
component's configuration file, so a change to the file restarts the component's pods (ADR-0052).
*/ -}}
{{- define "mediated-mailbox.podMetadata" -}}
{{- $v := index .root.Values .component -}}
{{- $annotations := deepCopy ($v.podAnnotations | default dict) -}}
{{- if .config }}
{{- $_ := set $annotations "checksum/config" (include "mediated-mailbox.configFile" . | sha256sum) -}}
{{- end }}
labels:
  {{- include "mediated-mailbox.labels" . | nindent 2 }}
  {{- with $v.podLabels }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- with $annotations }}
annotations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /*
The pod-level settings every pod the chart runs shares (ADR-0028, ADR-0052, ADR-0078). The security context is the
chart's top-level one with the component's own merged over it, so ADR-0028's defaults hold unless a value replaces
them. No pod is given a service account token unless its service account's automount value asks for one, since no
workload talks to the Kubernetes API. Service links are off, because Kubernetes' injected variables would carry the
configuration prefix and every deployable refuses a variable under the prefix it does not read.
*/ -}}
{{- define "mediated-mailbox.podSpec" -}}
{{- $v := index .root.Values .component -}}
{{- if $v.serviceAccount }}
serviceAccountName: {{ include "mediated-mailbox.serviceAccountName" . }}
automountServiceAccountToken: {{ $v.serviceAccount.automount }}
{{- else }}
automountServiceAccountToken: false
{{- end }}
enableServiceLinks: false
securityContext:
  {{- toYaml (mergeOverwrite (deepCopy .root.Values.podSecurityContext) ($v.podSecurityContext | default dict)) | nindent 2 }}
{{- with .root.Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $v.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $v.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with $v.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /* One container's image, security context and resources. */ -}}
{{- define "mediated-mailbox.container" -}}
{{- $v := index .root.Values .component -}}
image: {{ include "mediated-mailbox.image" . }}
imagePullPolicy: {{ $v.image.pullPolicy }}
securityContext:
  {{- toYaml (mergeOverwrite (deepCopy .root.Values.securityContext) ($v.securityContext | default dict)) | nindent 2 }}
{{- with $v.resources }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /*
The database section of a deployable's configuration (ADR-0078). The user is left to the deployable's default, its
own runtime role (ADR-0075), and the password is read from the file the deployable's own Secret mounts.
*/ -}}
{{- define "mediated-mailbox.databaseSection" -}}
{{- $db := .root.Values.database -}}
{{- $section := dict "host" (required "database.host is required" $db.host) "port" $db.port "name" (required "database.name is required" $db.name) "password_file" "/var/run/mediated-mailbox/database/password" -}}
{{- with $db.sslmode }}{{ $_ := set $section "sslmode" . }}{{ end -}}
{{- if $db.caSecret }}{{ $_ := set $section "sslrootcert" "/var/run/mediated-mailbox/database-ca/ca.crt" }}{{ end -}}
{{- toYaml $section }}
{{- end }}

{{- /* The paths of the mounted private keys, one file of the keyring each (ADR-0092). */ -}}
{{- define "mediated-mailbox.privateKeyFiles" -}}
{{- $files := list -}}
{{- range $i, $k := required "keys.privateKeys needs at least one key" .root.Values.keys.privateKeys -}}
{{- $files = append $files (printf "/var/run/mediated-mailbox/keys/private-%d.key" $i) -}}
{{- end -}}
{{- toYaml $files }}
{{- end }}

{{- /*
One deployable's configuration file, rendered from the structured values (ADR-0052, ADR-0078). It holds the
deployable's own section of the values, the shared inputs every deployable that declares them reads, the database
section, the key files and the shared scanner section, and the paths of the files the chart mounts. Each file holds
only keys its deployable declares, since every deployable refuses a key it does not.
*/ -}}
{{- define "mediated-mailbox.configFile" -}}
{{- $root := .root -}}
{{- $v := index $root.Values .component -}}
{{- $file := deepCopy ($v.config | default dict) -}}
{{- $_ := set $file "database" (include "mediated-mailbox.databaseSection" . | fromYaml) -}}
{{- $keys := dict "public" "/var/run/mediated-mailbox/keys/public.key" "private" (include "mediated-mailbox.privateKeyFiles" . | fromYamlArray) -}}
{{- if has .component (list "mediate" "backfill" "sync") }}
{{- $_ := set $file "credential" (dict "public_key_file" $keys.public "private_key_files" $keys.private) -}}
{{- with $root.Values.scanner }}{{ $_ := set $file "scanner" . }}{{ end -}}
{{- end }}
{{- if eq .component "mediate" }}
{{- $_ := set $file "token_file" "/var/run/mediated-mailbox/token/token" -}}
{{- if $v.tls.atIngress }}
{{- $_ := set $file "tls_at_ingress" true -}}
{{- else }}
{{- $_ := set $file "tls_cert" "/var/run/mediated-mailbox/tls/tls.crt" -}}
{{- $_ := set $file "tls_key" "/var/run/mediated-mailbox/tls/tls.key" -}}
{{- end }}
{{- end }}
{{- if eq .component "ui" }}
{{- $_ := set $file "seal_public_key_file" $keys.public -}}
{{- $_ := set $file "private_key_files" $keys.private -}}
{{- $_ := set $file "tls_cert" "/var/run/mediated-mailbox/tls/tls.crt" -}}
{{- $_ := set $file "tls_key" "/var/run/mediated-mailbox/tls/tls.key" -}}
{{- if $v.tokenKeySecret.name }}{{ $_ := set $file "token_key_file" "/var/run/mediated-mailbox/token-key/key" }}{{ end -}}
{{- with ($root.Values.sync.config | default dict).sync_interval }}{{ $_ := set $file "sync_interval" . }}{{ end -}}
{{- end }}
{{- toYaml $file }}
{{- end }}

{{- /* The argument naming a deployable's configuration file, and the file's mount. */ -}}
{{- define "mediated-mailbox.configArgs" -}}
- --config-file=/etc/mediated-mailbox/config.yaml
{{- end }}

{{- define "mediated-mailbox.configMount" -}}
- name: config
  mountPath: /etc/mediated-mailbox/config.yaml
  subPath: config.yaml
  readOnly: true
{{- end }}

{{- /* The volume that mounts only this deployable's file out of the one ConfigMap. */ -}}
{{- define "mediated-mailbox.configVolume" -}}
- name: config
  configMap:
    name: {{ include "mediated-mailbox.fullname" .root }}-config
    items:
    - key: {{ .component }}.yaml
      path: config.yaml
{{- end }}

{{- /* The mounts of a deployable's database password and, when one is named, the database's CA. */ -}}
{{- define "mediated-mailbox.databaseMounts" -}}
- name: database
  mountPath: /var/run/mediated-mailbox/database
  readOnly: true
{{- if .root.Values.database.caSecret }}
- name: database-ca
  mountPath: /var/run/mediated-mailbox/database-ca
  readOnly: true
{{- end }}
{{- end }}

{{- /* The volumes behind databaseMounts, from the deployable's own password Secret. */ -}}
{{- define "mediated-mailbox.databaseVolumes" -}}
{{- $password := (index .root.Values .component).passwordSecret -}}
- name: database
  secret:
    secretName: {{ required (printf "%s.passwordSecret.name is required" .component) $password.name }}
    defaultMode: 0440
    items:
    - key: {{ $password.key }}
      path: password
{{- with .root.Values.database.caSecret }}
- name: database-ca
  secret:
    secretName: {{ . }}
    defaultMode: 0440
    items:
    - key: {{ $.root.Values.database.caKey }}
      path: ca.crt
{{- end }}
{{- end }}

{{- /*
The key pair credentials are sealed to (ADR-0081, ADR-0092). Only the deployables that call a provider and the UI
mount it, and each private key is one file of the keyring.
*/ -}}
{{- define "mediated-mailbox.keysVolume" -}}
{{- $keys := .root.Values.keys -}}
- name: keys
  secret:
    secretName: {{ required "keys.secretName is required" $keys.secretName }}
    defaultMode: 0440
    items:
    - key: {{ $keys.publicKey }}
      path: public.key
    {{- range $i, $k := $keys.privateKeys }}
    - key: {{ $k }}
      path: private-{{ $i }}.key
    {{- end }}
{{- end }}

{{- define "mediated-mailbox.keysMount" -}}
- name: keys
  mountPath: /var/run/mediated-mailbox/keys
  readOnly: true
{{- end }}

{{- /*
The Ingress and the HTTPRoute of one exposed component, each rendered only when its value switches it on, in the
shape `helm create` gives them. HTTPRoute is the Gateway API's resource, so it is off by default and the chart assumes
nothing beyond core Kubernetes (ADR-0052). Either points at the component's Service port, which serves what the
deployable serves.
*/ -}}
{{- define "mediated-mailbox.ingress" -}}
{{- $v := index .root.Values .component -}}
{{- $name := include "mediated-mailbox.componentName" . -}}
{{- if $v.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ $name }}
  labels:
    {{- include "mediated-mailbox.labels" . | nindent 4 }}
  {{- with $v.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- with $v.ingress.className }}
  ingressClassName: {{ . }}
  {{- end }}
  {{- if $v.ingress.tls }}
  tls:
    {{- range $v.ingress.tls }}
    - hosts:
        {{- range .hosts }}
        - {{ . | quote }}
        {{- end }}
      secretName: {{ .secretName }}
    {{- end }}
  {{- end }}
  rules:
    {{- range $v.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            {{- with .pathType }}
            pathType: {{ . }}
            {{- end }}
            backend:
              service:
                name: {{ $name }}
                port:
                  number: {{ $v.service.port }}
          {{- end }}
    {{- end }}
{{- end }}
{{- end }}

{{- define "mediated-mailbox.httpRoute" -}}
{{- $v := index .root.Values .component -}}
{{- $name := include "mediated-mailbox.componentName" . -}}
{{- if $v.httpRoute.enabled }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ $name }}
  labels:
    {{- include "mediated-mailbox.labels" . | nindent 4 }}
  {{- with $v.httpRoute.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  parentRefs:
    {{- with $v.httpRoute.parentRefs }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
  {{- with $v.httpRoute.hostnames }}
  hostnames:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  rules:
    {{- range $v.httpRoute.rules }}
    {{- with .matches }}
    - matches:
      {{- toYaml . | nindent 8 }}
    {{- end }}
    {{- with .filters }}
      filters:
      {{- toYaml . | nindent 8 }}
    {{- end }}
      backendRefs:
        - name: {{ $name }}
          port: {{ $v.service.port }}
          weight: 1
    {{- end }}
{{- end }}
{{- end }}
