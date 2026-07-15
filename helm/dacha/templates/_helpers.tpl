{{- define "dacha.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "dacha.labels" -}}
app.kubernetes.io/name: {{ include "dacha.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "dacha.selectorLabels" -}}
app.kubernetes.io/name: {{ include "dacha.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "dacha.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "dacha.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "dacha.fullname" -}}
{{- include "dacha.fullname" . }}