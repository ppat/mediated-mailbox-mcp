{{- /*
Helpers every workload's template shares. Each covers one concern that is the same for every pod the chart runs:
the names and labels, the image reference, the hardening of ADR-0028, the database connection settings of ADR-0078,
and the mounted key pair of ADR-0081. A workload's own wiring stays in its own template.
*/ -}}

{{- /* The name of one component's objects, the release's name followed by the component's directory word. */ -}}
{{- define "mediated-mailbox.name" -}}
{{- printf "%s-%s" .root.Release.Name .component | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- /* The labels that select one component's pods. */ -}}
{{- define "mediated-mailbox.selector" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- /* Every label one component's objects carry. */ -}}
{{- define "mediated-mailbox.labels" -}}
{{ include "mediated-mailbox.selector" . }}
app.kubernetes.io/version: {{ .root.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end }}

{{- /*
One component's image. Every image moves in lockstep at the release's version, which the release workflow sets as
the chart's appVersion (ADR-0049), so a tag is set only to deploy another published version.
*/ -}}
{{- define "mediated-mailbox.image" -}}
{{- $image := .root.Values.image -}}
{{- printf "%s/mediated-mailbox-%s:%s" $image.registry .component ($image.tag | default .root.Chart.AppVersion) -}}
{{- end }}

{{- /*
The pod-level settings every pod the chart runs shares (ADR-0028, ADR-0052, ADR-0078). The images run as
distroless's nonroot user. No pod is given a service account token, since no workload talks to the Kubernetes API.
Service links are off, because Kubernetes' injected variables would carry the configuration prefix and every
deployable refuses a variable under the prefix it does not read.
*/ -}}
{{- define "mediated-mailbox.podSpec" -}}
automountServiceAccountToken: false
enableServiceLinks: false
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  fsGroup: 65532
  seccompProfile:
    type: RuntimeDefault
{{- with .root.Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /* The container-level hardening every container the chart runs shares (ADR-0028). */ -}}
{{- define "mediated-mailbox.containerSecurity" -}}
securityContext:
  allowPrivilegeEscalation: false
  privileged: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
{{- end }}

{{- /*
The environment variables that point a deployable at the database (ADR-0078). The user is left to each deployable's
default, its own runtime role (ADR-0075), and the password is read from the file the deployable's own Secret mounts.
*/ -}}
{{- define "mediated-mailbox.databaseEnv" -}}
{{- $db := .root.Values.database -}}
- name: MEDIATED_MAILBOX_DATABASE__HOST
  value: {{ required "database.host is required" $db.host | quote }}
- name: MEDIATED_MAILBOX_DATABASE__PORT
  value: {{ $db.port | quote }}
- name: MEDIATED_MAILBOX_DATABASE__NAME
  value: {{ required "database.name is required" $db.name | quote }}
{{- with $db.sslmode }}
- name: MEDIATED_MAILBOX_DATABASE__SSLMODE
  value: {{ . | quote }}
{{- end }}
{{- if $db.caSecret }}
- name: MEDIATED_MAILBOX_DATABASE__SSLROOTCERT
  value: /var/run/mediated-mailbox/database-ca/ca.crt
{{- end }}
- name: MEDIATED_MAILBOX_DATABASE__PASSWORD_FILE
  value: /var/run/mediated-mailbox/database/password
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

{{- /* The volumes behind databaseMounts. password is the deployable's own Secret reference, a name and a key. */ -}}
{{- define "mediated-mailbox.databaseVolumes" -}}
- name: database
  secret:
    secretName: {{ required (printf "%s.database.passwordSecret.name is required" .component) .password.name }}
    defaultMode: 0440
    items:
    - key: {{ .password.key }}
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
    {{- range $i, $k := required "keys.privateKeys needs at least one key" $keys.privateKeys }}
    - key: {{ $k }}
      path: private-{{ $i }}.key
    {{- end }}
{{- end }}

{{- define "mediated-mailbox.keysMount" -}}
- name: keys
  mountPath: /var/run/mediated-mailbox/keys
  readOnly: true
{{- end }}

{{- /* The paths of the mounted private keys, as the YAML list a configuration value takes from the environment. */ -}}
{{- define "mediated-mailbox.privateKeyFiles" -}}
{{- $files := list -}}
{{- range $i, $k := .root.Values.keys.privateKeys -}}
{{- $files = append $files (printf "/var/run/mediated-mailbox/keys/private-%d.key" $i) -}}
{{- end -}}
{{- toJson $files -}}
{{- end }}

{{- /*
The text of a deployable's configuration file, passed through as the operator wrote it (ADR-0078). For the
deployables that run the scanner the shared scanner section follows it, so every workload that masks or scans runs
the same section (ADR-0096). A scanner key written in both refuses the start as a duplicated key.
*/ -}}
{{- define "mediated-mailbox.configText" -}}
{{- $own := (index .root.Values .component).config | default "" -}}
{{- if .scanner -}}
{{- printf "%s\n%s" $own (.root.Values.scannerConfig | default "") -}}
{{- else -}}
{{- $own -}}
{{- end -}}
{{- end }}

{{- /* The ConfigMap holding a deployable's configuration file. */ -}}
{{- define "mediated-mailbox.configMap" -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "mediated-mailbox.name" . }}-config
  labels:
    {{- include "mediated-mailbox.labels" . | nindent 4 }}
data:
  config.yaml: {{ include "mediated-mailbox.configText" . | quote }}
{{- end }}

{{- /*
Whether a deployable has a configuration file. A file holding no document refuses the start (ADR-0078), so a
deployable whose text is empty gets no file, and its defaults, environment variables and flags carry everything.
*/ -}}
{{- define "mediated-mailbox.hasConfig" -}}
{{- if include "mediated-mailbox.configText" . | trim }}true{{ end -}}
{{- end }}

{{- /* The mount and volume of a deployable's configuration file. */ -}}
{{- define "mediated-mailbox.configMount" -}}
- name: config
  mountPath: /etc/mediated-mailbox
  readOnly: true
{{- end }}

{{- define "mediated-mailbox.configVolume" -}}
- name: config
  configMap:
    name: {{ include "mediated-mailbox.name" . }}-config
{{- end }}

{{- /*
Memory-backed scratch space for the workloads that hold bodies, so nothing that spills reaches a disk (ADR-0009).
The images' root filesystems are read-only, so this is the one writable path such a workload has.
*/ -}}
{{- define "mediated-mailbox.scratchVolume" -}}
- name: scratch
  emptyDir:
    medium: Memory
    sizeLimit: {{ (index .root.Values .component).scratchSize | default "64Mi" }}
{{- end }}

{{- define "mediated-mailbox.scratchMount" -}}
- name: scratch
  mountPath: /tmp
{{- end }}
