package common

type NodeRequest struct {
	Node        string `json:"node"`
	QueryType   int    `json:"query_type"`
	ForceDelete bool   `json:"force_delete"`
}
