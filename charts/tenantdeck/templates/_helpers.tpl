{{/*
Expand the name of the chart.
*/}}
{{- define "tenantdeck.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "tenantdeck.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "tenantdeck.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "tenantdeck.labels" -}}
helm.sh/chart: {{ include "tenantdeck.chart" . }}
{{ include "tenantdeck.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "tenantdeck.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenantdeck.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "tenantdeck.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "tenantdeck.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Full image reference: digest always wins over tag when both are set,
since a digest is what production should actually pin
(docs/spec/06-container-and-kubernetes-deployment.md "Pin base image
digests" - same principle applies to the app's own image). An empty
registry is omitted rather than left as a leading "/" - the E2E stack's
values (e2e/manifests/values-e2e.yaml) sets one to reference a
kind-loaded image by its bare local tag, which has no registry at all.
*/}}
{{- define "tenantdeck.image" -}}
{{- $repo := .Values.image.registry | default "" -}}
{{- if $repo -}}{{- $repo = printf "%s/%s" $repo .Values.image.repository -}}{{- else -}}{{- $repo = .Values.image.repository -}}{{- end -}}
{{- if .Values.image.digest -}}
{{ $repo }}@{{ .Values.image.digest }}
{{- else -}}
{{ $repo }}:{{ .Values.image.tag | default .Chart.AppVersion }}
{{- end -}}
{{- end }}
