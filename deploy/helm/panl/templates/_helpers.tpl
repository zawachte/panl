{{- define "panl.name" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "panl.labels" -}}
app.kubernetes.io/name: {{ include "panl.selectorName" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- define "panl.agentSecret" -}}
{{- .Values.agent.existingSecret | default (printf "%s-agent" (include "panl.name" .)) -}}
{{- end -}}

{{- define "panl.selectorName" -}}
{{- .Values.nameOverride | default .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
