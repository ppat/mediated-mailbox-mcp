{{/*
Small fragments the templates repeat: names, labels, the image reference, the security contexts, and the projected
secret sources more than one pod mounts.
*/}}

{{- define "mm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "mm.fullname" -}}
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

{{- define "mm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* The name of one component's objects, such as release-mediated-mailbox-worker. Takes (list $ "worker"). */}}
{{- define "mm.componentName" -}}
{{- printf "%s-%s" (include "mm.fullname" (index . 0) | trunc 50 | trimSuffix "-") (index . 1) }}
{{- end }}

{{/* Selector labels are only the name, the instance and the component, which never change across releases. */}}
{{- define "mm.selectorLabels" -}}
{{- $ctx := index . 0 }}
app.kubernetes.io/name: {{ include "mm.name" $ctx }}
app.kubernetes.io/instance: {{ $ctx.Release.Name }}
app.kubernetes.io/component: {{ index . 1 }}
{{- end }}

{{/* The standard labels of one component's objects. Takes (list $ "worker"). */}}
{{- define "mm.labels" -}}
{{- $ctx := index . 0 }}
helm.sh/chart: {{ include "mm.chart" $ctx }}
{{ include "mm.selectorLabels" . }}
app.kubernetes.io/version: {{ $ctx.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ $ctx.Release.Service }}
app.kubernetes.io/part-of: {{ include "mm.name" $ctx }}
{{- with $ctx.Values.global.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/* The standard labels of an object that belongs to no one component, such as the ConfigMap. */}}
{{- define "mm.sharedLabels" -}}
helm.sh/chart: {{ include "mm.chart" . }}
app.kubernetes.io/name: {{ include "mm.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: {{ include "mm.name" . }}
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
An image reference. The tag defaults to the chart's appVersion, so every image moves in lockstep with the chart
(ADR-0049), and a digest, when given, pins the image whatever the tag. Takes (list $ .Values.<component>.image).
*/}}
{{- define "mm.image" -}}
{{- $ctx := index . 0 }}
{{- $image := index . 1 }}
{{- $ref := printf "%s/%s:%s" $ctx.Values.global.image.registry $image.repository (default $ctx.Chart.AppVersion $image.tag) }}
{{- if $image.digest }}
{{- $ref = printf "%s@%s" $ref $image.digest }}
{{- end }}
{{- $ref }}
{{- end }}

{{/* A component's ServiceAccount name. Takes (list $ "worker" .Values.worker.serviceAccount). */}}
{{- define "mm.serviceAccountName" -}}
{{- $sa := index . 2 }}
{{- if $sa.create }}
{{- default (include "mm.componentName" (list (index . 0) (index . 1))) $sa.name }}
{{- else }}
{{- default "default" $sa.name }}
{{- end }}
{{- end }}

{{/* The pod and container security contexts: the restricted defaults under global, with a component's overrides. */}}
{{- define "mm.podSecurityContext" -}}
{{- toYaml (mergeOverwrite (deepCopy (index . 0).Values.global.podSecurityContext) ((index . 1).podSecurityContext | default dict)) }}
{{- end }}

{{- define "mm.securityContext" -}}
{{- toYaml (mergeOverwrite (deepCopy (index . 0).Values.global.securityContext) ((index . 1).securityContext | default dict)) }}
{{- end }}

{{/*
The ports every process listens on. The chart renders them into each process's file, so the container ports, the
probes and the Services read the same number.
*/}}
{{- define "mm.port.http" -}}8443{{- end }}
{{- define "mm.port.metrics" -}}8080{{- end }}

{{/* Where the files a pod reads are mounted. */}}
{{- define "mm.configDir" -}}/etc/mediated-mailbox/config{{- end }}
{{- define "mm.secretsDir" -}}/etc/mediated-mailbox/secrets{{- end }}

{{/* The Secret the chart creates for whichever of the mediator's bearer token and the UI's request token key no Secret is named for. */}}
{{- define "mm.accessCredentialsSecretName" -}}
{{- printf "%s-access-credentials" (include "mm.fullname" .) }}
{{- end }}

{{/* The Secret holding the mediator's bearer token, the user's or the chart's own. */}}
{{- define "mm.mediatorBearerTokenSecretName" -}}
{{- default (include "mm.accessCredentialsSecretName" .) .Values.secrets.mediatorBearerToken.existingSecret }}
{{- end }}

{{/* The Secret holding the UI's request token key, the user's or the chart's own. */}}
{{- define "mm.uiRequestTokenKeySecretName" -}}
{{- default (include "mm.accessCredentialsSecretName" .) .Values.secrets.uiRequestTokenKey.existingSecret }}
{{- end }}

{{/*
The projected secret sources more than one pod mounts. Each lists only the keys a process reads, under a fixed path
the rendered configuration names (ADR-0079).
*/}}
{{- define "mm.credentialSealingKeyPairSource" -}}
{{- $k := .Values.secrets.credentialSealingKeyPair }}
- secret:
    name: {{ required "secrets.credentialSealingKeyPair.existingSecret is required: the Secret holding the key pair keygen writes" $k.existingSecret }}
    items:
    - key: {{ $k.secretKeys.publicKey }}
      path: credential-sealing-key-pair/{{ $k.secretKeys.publicKey }}
    {{- range $k.secretKeys.privateKeys }}
    - key: {{ . }}
      path: credential-sealing-key-pair/{{ . }}
    {{- end }}
{{- end }}

{{- define "mm.databaseCaCertificateSource" -}}
{{- with .Values.secrets.databaseCaCertificate }}
{{- if .existingSecret }}
- secret:
    name: {{ .existingSecret }}
    items:
    - key: {{ .secretKeys.certificate }}
      path: ca/ca.crt
{{- end }}
{{- end }}
{{- end }}

{{/* Whether a Prometheus Operator resource renders: auto renders it where the cluster serves its kind. Takes (list $ enabled "group/version/Kind"). */}}
{{- define "mm.monitoringEnabled" -}}
{{- $ctx := index . 0 }}
{{- $enabled := index . 1 }}
{{- if eq (toString $enabled) "auto" }}
{{- if $ctx.Capabilities.APIVersions.Has (index . 2) }}true{{ end }}
{{- else if $enabled }}true{{ end }}
{{- end }}
