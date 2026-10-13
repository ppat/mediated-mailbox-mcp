{{/*
Each process's configuration file, rendered from the structured values (ADR-0078). The ConfigMap holds these, and
each Deployment's checksum hashes its own, so a change restarts exactly the processes whose file it changes. The
chart owns the listen addresses and every file path, the shared config block gives what several processes read
once, and each process's own block the rest. The values schema refuses a chart-owned key in a process's own block,
so no key is written twice.
*/}}

{{/* The database server's settings, shared by every process. */}}
{{- define "mm.config.databaseServer" -}}
{{- $d := .Values.database }}
{{- $s := dict "host" (required "database.host is required" $d.host) "port" $d.port "name" (required "database.name is required" $d.name) "sslmode" $d.sslmode }}
{{- if .Values.secrets.databaseCaCertificate.existingSecret }}
{{- $_ := set $s "sslrootcert" (printf "%s/ca/ca.crt" (include "mm.secretsDir" .)) }}
{{- end }}
{{- toYaml $s }}
{{- end }}

{{/* The credential section, the same keys in every process that reads the key pair (ADR-0079, ADR-0081). */}}
{{- define "mm.config.credential" -}}
{{- $k := .Values.secrets.credentialSealingKeyPair }}
{{- $files := list }}
{{- range $k.secretKeys.privateKeys }}
{{- $files = append $files (printf "%s/credential-sealing-key-pair/%s" (include "mm.secretsDir" $) .) }}
{{- end }}
{{- toYaml (dict "public_key_file" (printf "%s/credential-sealing-key-pair/%s" (include "mm.secretsDir" .) $k.secretKeys.publicKey) "private_key_files" $files) }}
{{- end }}

{{/* A process's database section: the server, its role and pool size when set, and its password file. Takes (list $ .Values.<proc>.database). */}}
{{- define "mm.config.database" -}}
{{- $ := index . 0 }}
{{- $db := include "mm.config.databaseServer" $ | fromYaml }}
{{- with (index . 1).user }}
{{- $_ := set $db "user" . }}
{{- end }}
{{- with (index . 1).poolSize }}
{{- $_ := set $db "pool_size" . }}
{{- end }}
{{- $_ := set $db "password_file" (printf "%s/database/password" (include "mm.secretsDir" $)) }}
{{- toYaml $db }}
{{- end }}

{{- define "mm.config.api" -}}
{{- $v := .Values.api }}
{{- $c := deepCopy $v.config }}
{{- $_ := set $c "log_level" (default .Values.sharedConfig.log_level (index $v.config "log_level")) }}
{{- if .Values.sharedConfig.scanner }}
{{- $_ := set $c "scanner" .Values.sharedConfig.scanner }}
{{- end }}
{{- $_ := set $c "database" (include "mm.config.database" (list . $v.database) | fromYaml) }}
{{- $_ := set $c "credential" (include "mm.config.credential" . | fromYaml) }}
{{- $_ := set $c "listen" (printf ":%s" (include "mm.port.http" .)) }}
{{- $_ := set $c "probe_listen" (printf ":%s" (include "mm.port.metrics" .)) }}
{{- $_ := set $c "token_file" (printf "%s/bearer-token" (include "mm.secretsDir" .)) }}
{{- if .Values.secrets.apiTls.existingSecret }}
{{- $_ := set $c "tls_cert" (printf "%s/tls/tls.crt" (include "mm.secretsDir" .)) }}
{{- $_ := set $c "tls_key" (printf "%s/tls/tls.key" (include "mm.secretsDir" .)) }}
{{- end }}
{{- toYaml $c }}
{{- end }}

{{- define "mm.config.ui" -}}
{{- $v := .Values.ui }}
{{- $c := deepCopy $v.config }}
{{- $_ := set $c "log_level" (default .Values.sharedConfig.log_level (index $v.config "log_level")) }}
{{- $_ := set $c "sync_interval" .Values.sharedConfig.sync_interval }}
{{- $_ := set $c "database" (include "mm.config.database" (list . $v.database) | fromYaml) }}
{{- $_ := set $c "credential" (include "mm.config.credential" . | fromYaml) }}
{{- $_ := set $c "listen" (printf ":%s" (include "mm.port.http" .)) }}
{{- $_ := set $c "probe_listen" (printf ":%s" (include "mm.port.metrics" .)) }}
{{- $_ := set $c "token_key_file" (printf "%s/request-token-key" (include "mm.secretsDir" .)) }}
{{- if .Values.secrets.uiTls.existingSecret }}
{{- $_ := set $c "tls_cert" (printf "%s/tls/tls.crt" (include "mm.secretsDir" .)) }}
{{- $_ := set $c "tls_key" (printf "%s/tls/tls.key" (include "mm.secretsDir" .)) }}
{{- end }}
{{- toYaml $c }}
{{- end }}

{{/* The worker's file: the server once, and under database a block per job kind with its role, pool and password file. */}}
{{- define "mm.config.worker" -}}
{{- $c := deepCopy .Values.worker.config }}
{{- $_ := set $c "log_level" (default .Values.sharedConfig.log_level (index .Values.worker.config "log_level")) }}
{{- if .Values.sharedConfig.scanner }}
{{- $_ := set $c "scanner" .Values.sharedConfig.scanner }}
{{- end }}
{{- $db := include "mm.config.databaseServer" . | fromYaml }}
{{- range $kind, $jk := .Values.worker.jobKinds }}
{{- $role := dict "password_file" (printf "%s/database/%s/password" (include "mm.secretsDir" $) $kind) }}
{{- with $jk.database.user }}
{{- $_ := set $role "user" . }}
{{- end }}
{{- with $jk.database.poolSize }}
{{- $_ := set $role "pool_size" . }}
{{- end }}
{{- $_ := set $db $kind $role }}
{{- $section := deepCopy $jk.config }}
{{- if eq $kind "sync" }}
{{- $_ := set $section "sync_interval" $.Values.sharedConfig.sync_interval }}
{{- end }}
{{- if $section }}
{{- $_ := set $c $kind $section }}
{{- end }}
{{- end }}
{{- $_ := set $c "database" $db }}
{{- $_ := set $c "credential" (include "mm.config.credential" . | fromYaml) }}
{{- $_ := set $c "probe_listen" (printf ":%s" (include "mm.port.metrics" .)) }}
{{- toYaml $c }}
{{- end }}
