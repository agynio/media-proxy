{{- define "media-proxy.configureEnv" -}}
{{- $env := list -}}

{{- $listenAddr := trimAll " \n\t" (default ":8080" .Values.mediaProxy.listenAddr) -}}
{{- if $listenAddr }}
{{- $env = append $env (dict "name" "LISTEN_ADDR" "value" $listenAddr) -}}
{{- end }}

{{- $issuer := trimAll " \n\t" (default "" .Values.mediaProxy.oidcIssuerUrl) -}}
{{- $env = append $env (dict "name" "OIDC_ISSUER_URL" "value" $issuer) -}}

{{- $clientId := trimAll " \n\t" (default "" .Values.mediaProxy.oidcClientId) -}}
{{- $env = append $env (dict "name" "OIDC_CLIENT_ID" "value" $clientId) -}}

{{- $usersTarget := trimAll " \n\t" (default "users:50051" .Values.mediaProxy.usersGrpcTarget) -}}
{{- $env = append $env (dict "name" "USERS_GRPC_TARGET" "value" $usersTarget) -}}

{{- $filesTarget := trimAll " \n\t" (default "files:50051" .Values.mediaProxy.filesGrpcTarget) -}}
{{- $env = append $env (dict "name" "FILES_GRPC_TARGET" "value" $filesTarget) -}}

{{- $corsOrigin := trimAll " \n\t" (default "https://agyn.dev" .Values.mediaProxy.corsAllowedOrigin) -}}
{{- if $corsOrigin }}
{{- $env = append $env (dict "name" "CORS_ALLOWED_ORIGIN" "value" $corsOrigin) -}}
{{- end }}

{{- $maxResponse := int (default 52428800 .Values.mediaProxy.maxResponseSize) -}}
{{- $env = append $env (dict "name" "MAX_RESPONSE_SIZE" "value" (printf "%d" $maxResponse)) -}}

{{- $requestTimeout := trimAll " \n\t" (default "30s" .Values.mediaProxy.requestTimeout) -}}
{{- if $requestTimeout }}
{{- $env = append $env (dict "name" "REQUEST_TIMEOUT" "value" $requestTimeout) -}}
{{- end }}

{{- $maxRedirects := int (default 3 .Values.mediaProxy.maxRedirects) -}}
{{- $env = append $env (dict "name" "MAX_REDIRECTS" "value" (printf "%d" $maxRedirects)) -}}

{{- $maxImageSize := int (default 4096 .Values.mediaProxy.maxImageSize) -}}
{{- $env = append $env (dict "name" "MAX_IMAGE_SIZE" "value" (printf "%d" $maxImageSize)) -}}

{{- $userEnv := .Values.env | default (list) -}}
{{- $_ := set .Values "env" (concat $env $userEnv) -}}
{{- end -}}
