package common

const (
	MetricsNamePingLatency       = `ping_latency_milliseconds`
	MetricsNamePingPacketDrop    = `ping_packetDrop_rate`
	MetricsNamePingTargetSuccess = `ping_target_success`

	MetricsNameHTTPResolveDurationMilliseconds  = `http_resolve_duration_milliseconds`
	MetricsNameHTTPTLSDurationMilliseconds      = `http_tls_duration_milliseconds`
	MetricsNameHTTPConnectDurationMilliseconds  = `http_connect_duration_milliseconds`
	MetricsNameHTTPProcessDurationMilliseconds  = `http_processing_duration_milliseconds`
	MetricsNameHTTPTransferDurationMilliseconds = `http_transfer_duration_milliseconds`
	MetricsNameHTTPInterfaceSuccess             = `http_interface_success`

	MetricsNameTCPConnectDurationMilliseconds = `tcp_connect_duration_milliseconds`
	MetricsNameTCPConnectSuccess              = `tcp_connect_success`

	MetricsNameDNSLookupDurationMilliseconds = `dns_lookup_duration_milliseconds`
	MetricsNameDNSLookupSuccess              = `dns_lookup_success`
	MetricsNameDNSAnswerCount                = `dns_answer_count`

	MetricsNameTLSHandshakeDurationMilliseconds = `tls_handshake_duration_milliseconds`
	MetricsNameTLSConnectSuccess                = `tls_connect_success`
	MetricsNameTLSCertificateValid              = `tls_certificate_valid`
	MetricsNameTLSCertificateExpirySeconds      = `tls_certificate_expiry_seconds`
)

type Result struct {
	WorkerName    string  `json:"workerName"`
	MetricsName   string  `json:"metricsName"`
	TargetAddress string  `json:"targetAddress"`
	SourceRegion  string  `json:"sourceRegion"`
	TargetRegion  string  `json:"targetRegion"`
	Type          string  `json:"type"`
	TimeStamp     int64   `json:"timeStamp"`
	Value         float32 `json:"value"`
}

type ResultPushRequest struct {
	Results []*Result `json:"results,omitempty"`
}

type ResultPushResponse struct {
	SuccessNum int32 `json:"successNum,omitempty"`
}

type Targets struct {
	Type            string            `json:"type"`
	Region          string            `json:"region"`
	Target          []string          `json:"target"`
	IntervalSeconds int               `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int               `json:"timeoutSeconds,omitempty"`
	Options         map[string]string `json:"options,omitempty"`
}

type TargetGetRequest struct {
	LocalRegion string `json:"localRegion"`
	LocalIp     string `json:"localIp"`
}

type TargetGetResponse struct {
	Targets []*Targets `json:"targets"`
}
