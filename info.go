package gateway

// Info gateway详情
type Info struct {
	Qps        int32        `json:"qps"`
	Total      uint64       `json:"total"`
	MethodList []MethodInfo `json:"method_list"`
}
