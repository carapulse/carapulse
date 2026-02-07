{{/*
Expand the name of the chart.
*/}}
{{- define "carapulse.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If release name contains chart name it will be used as
a full name.
*/}}
{{- define "carapulse.fullname" -}}
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
{{- define "carapulse.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "carapulse.labels" -}}
helm.sh/chart: {{ include "carapulse.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Values.global.image.tag | default .Chart.AppVersion | quote }}
app.kubernetes.io/part-of: carapulse
{{- end }}

{{/*
Selector labels for a given component.
Usage: include "carapulse.selectorLabels" (dict "context" . "component" "gateway")
*/}}
{{- define "carapulse.selectorLabels" -}}
app.kubernetes.io/name: {{ include "carapulse.name" .context }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Component labels — common labels plus selector labels.
Usage: include "carapulse.componentLabels" (dict "context" . "component" "gateway")
*/}}
{{- define "carapulse.componentLabels" -}}
{{ include "carapulse.labels" .context }}
{{ include "carapulse.selectorLabels" (dict "context" .context "component" .component) }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "carapulse.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "carapulse.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Resolve image reference for a component.
Usage: include "carapulse.image" (dict "global" .Values.global "image" .Values.gateway.image)
*/}}
{{- define "carapulse.image" -}}
{{- $tag := .global.image.tag | default .appVersion -}}
{{- printf "%s/%s:%s" .global.image.registry .image.repository $tag }}
{{- end }}
