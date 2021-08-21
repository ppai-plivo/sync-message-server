package main

// FrontendResponse is response returned to the client by the proxy.
type FrontendResponse struct {
	MessageUUID  string `json:"message_uuid" schema:"MessageUUID,required"`
	MessageState string `json:"message_state" schema:"Status,required"`
	MessageTime  string `json:"message_time" schema:"MessageTime,required"`
	SentTime     string `json:"sent_time,omitempty" schema:"SentTime,required"`
	TotalRate    string `json:"total_rate" schema:"TotalRate,required"`
	TotalAmount  string `json:"total_amount" schema:"TotalAmount,required"`
	Units        int    `json:"units" schema:"Units,required"`
	ErrorCode    string `json:"error_code" schema:"ErrorCode"`
}

// backendRequest is accepted from client and forwarded to Plivo API.
type backendRequest struct {
	Src           string      `json:"src,omitempty"`
	Dst           string      `json:"dst,omitempty"`
	Text          string      `json:"text,omitempty"`
	Type          string      `json:"type,omitempty"`
	URL           string      `json:"url,omitempty"`
	Method        string      `json:"method,omitempty"`
	Trackable     bool        `json:"trackable,omitempty"`
	Log           interface{} `json:"log,omitempty"`
	MediaUrls     []string    `json:"media_urls,omitempty"`
	MediaIds      []string    `json:"media_ids,omitempty"`
	PowerpackUUID string      `json:"powerpack_uuid,omitempty"`
}

// backendResponse is the response from Plivo API.
type backendResponse struct {
	Message     string   `json:"message"`
	MessageUUID []string `json:"message_uuid"`
}
