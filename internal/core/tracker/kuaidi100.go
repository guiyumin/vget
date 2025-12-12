package tracker

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	Kuaidi100APIURL         = "https://poll.kuaidi100.com/poll/query.do"
	Kuaidi100AutoNumberURL  = "http://www.kuaidi100.com/autonumber/auto"
	Kuaidi100DeliveryTimeURL = "https://api.kuaidi100.com/label/order?method=time"
)

// Kuaidi100Config holds the API credentials
type Kuaidi100Config struct {
	Key      string // Authorization key (授权key)
	Customer string // Customer ID (查询公司编号)
	Secret   string // Secret for delivery time API (授权secret)
}

// Kuaidi100Tracker implements package tracking via kuaidi100.com API
type Kuaidi100Tracker struct {
	config Kuaidi100Config
	client *http.Client
}

// NewKuaidi100Tracker creates a new tracker instance
func NewKuaidi100Tracker(key, customer string) *Kuaidi100Tracker {
	return &Kuaidi100Tracker{
		config: Kuaidi100Config{
			Key:      key,
			Customer: customer,
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewKuaidi100TrackerWithSecret creates a new tracker instance with secret for delivery time API
func NewKuaidi100TrackerWithSecret(key, customer, secret string) *Kuaidi100Tracker {
	return &Kuaidi100Tracker{
		config: Kuaidi100Config{
			Key:      key,
			Customer: customer,
			Secret:   secret,
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSecret sets the secret for delivery time API
func (t *Kuaidi100Tracker) SetSecret(secret string) {
	t.config.Secret = secret
}

// QueryParam represents the query parameters for kuaidi100 API
type QueryParam struct {
	Com      string `json:"com"`               // Courier company code (e.g., "yuantong", "shunfeng")
	Num      string `json:"num"`               // Tracking number
	Phone    string `json:"phone,omitempty"`   // Phone number (required for some couriers like SF)
	From     string `json:"from,omitempty"`    // Origin city
	To       string `json:"to,omitempty"`      // Destination city
	Resultv2 string `json:"resultv2"`          // Enable district parsing (1=enabled)
	Show     string `json:"show"`              // Response format: 0=json, 1=xml, 2=html, 3=text
	Order    string `json:"order"`             // Sort order: desc (newest first), asc (oldest first)
}

// TrackingResponse represents the API response
type TrackingResponse struct {
	Message   string         `json:"message"`   // Error message if any
	State     string         `json:"state"`     // Tracking state code
	Status    string         `json:"status"`    // Status code (200=success)
	Condition string         `json:"condition"` // Current condition
	IsCheck   string         `json:"ischeck"`   // Whether delivered (1=yes)
	Com       string         `json:"com"`       // Courier company code
	Nu        string         `json:"nu"`        // Tracking number
	Data      []TrackingData `json:"data"`      // Tracking events
}

// TrackingData represents a single tracking event
type TrackingData struct {
	Context    string `json:"context"`    // Event description
	Time       string `json:"time"`       // Event time (formatted)
	Ftime      string `json:"ftime"`      // Event time (formatted, alternative)
	Status     string `json:"status"`     // Status at this point
	AreaCode   string `json:"areaCode"`   // Area code
	AreaName   string `json:"areaName"`   // Area name
	AreaCenter string `json:"areaCenter"` // Area center coordinates
	Location   string `json:"location"`   // Location description
}

// StateDescription returns human-readable state description
func (r *TrackingResponse) StateDescription() string {
	states := map[string]string{
		"0":  "在途",       // In transit
		"1":  "揽收",       // Picked up
		"2":  "疑难",       // Problem
		"3":  "已签收",      // Delivered
		"4":  "退签",       // Rejected
		"5":  "派件中",      // Out for delivery
		"6":  "退回",       // Returned
		"7":  "转投",       // Redirected
		"10": "待清关",      // Pending customs
		"11": "清关中",      // Customs processing
		"12": "已清关",      // Customs cleared
		"13": "清关异常",     // Customs exception
		"14": "收件人拒签",    // Recipient refused
	}
	if desc, ok := states[r.State]; ok {
		return desc
	}
	return "未知状态"
}

// IsDelivered returns true if package has been delivered
func (r *TrackingResponse) IsDelivered() bool {
	return r.IsCheck == "1" || r.State == "3"
}

// Track queries the tracking info for a package
func (t *Kuaidi100Tracker) Track(courierCode, trackingNumber string) (*TrackingResponse, error) {
	return t.TrackWithPhone(courierCode, trackingNumber, "")
}

// TrackWithPhone queries tracking info with phone number (required for some couriers like SF Express)
func (t *Kuaidi100Tracker) TrackWithPhone(courierCode, trackingNumber, phone string) (*TrackingResponse, error) {
	// Build query parameters
	param := QueryParam{
		Com:      courierCode,
		Num:      trackingNumber,
		Phone:    phone,
		Resultv2: "1",
		Show:     "0", // JSON format
		Order:    "desc",
	}

	paramJSON, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Calculate sign: MD5(param + key + customer) -> uppercase
	signStr := string(paramJSON) + t.config.Key + t.config.Customer
	hash := md5.Sum([]byte(signStr))
	sign := strings.ToUpper(hex.EncodeToString(hash[:]))

	// Build POST data
	formData := url.Values{}
	formData.Set("customer", t.config.Customer)
	formData.Set("param", string(paramJSON))
	formData.Set("sign", sign)

	// Send request
	resp, err := t.client.PostForm(Kuaidi100APIURL, formData)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result TrackingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	// Check for API errors
	if result.Status != "200" && result.Status != "" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return &result, nil
}

// AutoNumberResponse represents the auto number detection API response
type AutoNumberResponse struct {
	ComCode  string `json:"comCode"`  // Courier company code
	NoCount  int    `json:"noCount"`  // Match count
	NoPre    string `json:"noPre"`    // Number prefix
	StartTime string `json:"startTime"` // Start time
}

// AutoNumber detects the courier company from a tracking number
// Returns a list of possible courier codes
func (t *Kuaidi100Tracker) AutoNumber(trackingNumber string) ([]AutoNumberResponse, error) {
	formData := url.Values{}
	formData.Set("key", t.config.Key)
	formData.Set("num", trackingNumber)

	resp, err := t.client.PostForm(Kuaidi100AutoNumberURL, formData)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result []AutoNumberResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return result, nil
}

// DeliveryTimeParam represents the parameters for delivery time estimation
type DeliveryTimeParam struct {
	Kuaidicom string `json:"kuaidicom"`         // Courier company code
	From      string `json:"from"`              // Origin address (must include 3+ levels, e.g., 广东深圳市南山区)
	To        string `json:"to"`                // Destination address (must include 3+ levels)
	OrderTime string `json:"orderTime"`         // Order time, format: yyyy-MM-dd HH:mm:ss
	ExpType   string `json:"expType,omitempty"` // Product type (e.g., 特惠送, 标快)
}

// DeliveryTimeResponse represents the delivery time estimation API response
type DeliveryTimeResponse struct {
	Result     bool   `json:"result"`     // Whether successful
	ReturnCode string `json:"returnCode"` // Return code
	Message    string `json:"message"`    // Error message if any
	Data       *DeliveryTimeData `json:"data,omitempty"`
}

// DeliveryTimeData contains the estimated delivery time info
type DeliveryTimeData struct {
	ArriveTime     string `json:"arriveTime"`     // Estimated arrival time
	SortingName    string `json:"sortingName"`    // Sorting center name
	PickupTime     string `json:"pickupTime"`     // Estimated pickup time
	Hour           string `json:"hour"`           // Estimated hours
	Day            string `json:"day"`            // Estimated days
	ExpectTime     string `json:"expectTime"`     // Expected delivery time range
	SecondDayArrive bool   `json:"secondDayArrive"` // Whether arrives next day
}

// EstimateDeliveryTime estimates the delivery time for a shipment
// Requires secret to be configured
func (t *Kuaidi100Tracker) EstimateDeliveryTime(param DeliveryTimeParam) (*DeliveryTimeResponse, error) {
	if t.config.Secret == "" {
		return nil, fmt.Errorf("secret is required for delivery time estimation, set express.kuaidi100.secret in config")
	}

	// Set default order time if not provided
	if param.OrderTime == "" {
		param.OrderTime = time.Now().Format("2006-01-02 15:04:05")
	}

	paramJSON, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Get current timestamp in milliseconds
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Calculate sign: MD5(param + t + key + secret) -> uppercase
	signStr := string(paramJSON) + timestamp + t.config.Key + t.config.Secret
	hash := md5.Sum([]byte(signStr))
	sign := strings.ToUpper(hex.EncodeToString(hash[:]))

	// Build POST data
	formData := url.Values{}
	formData.Set("param", string(paramJSON))
	formData.Set("key", t.config.Key)
	formData.Set("t", timestamp)
	formData.Set("sign", sign)

	resp, err := t.client.PostForm(Kuaidi100DeliveryTimeURL, formData)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result DeliveryTimeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	if !result.Result {
		return nil, fmt.Errorf("API error: %s (code: %s)", result.Message, result.ReturnCode)
	}

	return &result, nil
}

// CourierInfo contains courier code and name
type CourierInfo struct {
	Code string // kuaidi100 API code
	Name string // Chinese name
}

// CourierCodes maps aliases/codes to kuaidi100 courier info
// Source: https://github.com/simman/Kuaidi100 and https://www.kuaidi100.com/all/
var CourierCodes = map[string]CourierInfo{
	// 顺丰
	"sf":       {Code: "shunfeng", Name: "顺丰速运"},
	"shunfeng": {Code: "shunfeng", Name: "顺丰速运"},
	"顺丰":      {Code: "shunfeng", Name: "顺丰速运"},

	// 圆通
	"yt":       {Code: "yuantong", Name: "圆通速递"},
	"yuantong": {Code: "yuantong", Name: "圆通速递"},
	"圆通":      {Code: "yuantong", Name: "圆通速递"},

	// 申通
	"sto":      {Code: "shentong", Name: "申通快递"},
	"shentong": {Code: "shentong", Name: "申通快递"},
	"申通":      {Code: "shentong", Name: "申通快递"},

	// 中通
	"zto":       {Code: "zhongtong", Name: "中通快递"},
	"zhongtong": {Code: "zhongtong", Name: "中通快递"},
	"中通":       {Code: "zhongtong", Name: "中通快递"},

	// 韵达
	"yd":    {Code: "yunda", Name: "韵达快递"},
	"yunda": {Code: "yunda", Name: "韵达快递"},
	"韵达":   {Code: "yunda", Name: "韵达快递"},

	// 极兔
	"jt":        {Code: "jtexpress", Name: "极兔速递"},
	"jitu":      {Code: "jtexpress", Name: "极兔速递"},
	"jtexpress": {Code: "jtexpress", Name: "极兔速递"},
	"极兔":       {Code: "jtexpress", Name: "极兔速递"},

	// 京东
	"jd":   {Code: "jd", Name: "京东物流"},
	"京东":  {Code: "jd", Name: "京东物流"},

	// EMS
	"ems": {Code: "ems", Name: "EMS"},

	// 邮政
	"yzgn":   {Code: "youzhengguonei", Name: "邮政国内"},
	"youzheng": {Code: "youzhengguonei", Name: "邮政国内"},
	"邮政":    {Code: "youzhengguonei", Name: "邮政国内"},

	// 德邦
	"dbwl":       {Code: "debangwuliu", Name: "德邦物流"},
	"debang":     {Code: "debangwuliu", Name: "德邦物流"},
	"debangwuliu": {Code: "debangwuliu", Name: "德邦物流"},
	"德邦":        {Code: "debangwuliu", Name: "德邦物流"},

	// 安能
	"anneng":     {Code: "annengwuliu", Name: "安能物流"},
	"annengwuliu": {Code: "annengwuliu", Name: "安能物流"},
	"安能":        {Code: "annengwuliu", Name: "安能物流"},

	// 百世/汇通
	"best":         {Code: "huitongkuaidi", Name: "百世快递"},
	"huitong":      {Code: "huitongkuaidi", Name: "百世快递"},
	"huitongkuaidi": {Code: "huitongkuaidi", Name: "百世快递"},
	"百世":          {Code: "huitongkuaidi", Name: "百世快递"},

	// 跨越
	"kuayue": {Code: "kuayue", Name: "跨越速运"},
	"跨越":    {Code: "kuayue", Name: "跨越速运"},

	// 国际快递
	"ups":   {Code: "ups", Name: "UPS"},
	"fedex": {Code: "fedex", Name: "FedEx"},
	"dhl":   {Code: "dhl", Name: "DHL"},
	"tnt":   {Code: "tnt", Name: "TNT"},
	"usps":  {Code: "usps", Name: "USPS"},

	// 菜鸟
	"cainiao": {Code: "cainiao", Name: "菜鸟"},
	"菜鸟":     {Code: "cainiao", Name: "菜鸟"},
}

// GetCourierCode returns the kuaidi100 courier code for an alias
func GetCourierCode(alias string) string {
	alias = strings.ToLower(alias)
	if info, ok := CourierCodes[alias]; ok {
		return info.Code
	}
	// Return as-is if no alias found (might be direct kuaidi100 code)
	return alias
}

// GetCourierInfo returns the courier info for an alias, or nil if not found
func GetCourierInfo(alias string) *CourierInfo {
	alias = strings.ToLower(alias)
	if info, ok := CourierCodes[alias]; ok {
		return &info
	}
	return nil
}

// ListCouriers returns a list of common courier codes for display
func ListCouriers() []CourierInfo {
	// Return deduplicated list of common couriers
	return []CourierInfo{
		{Code: "shunfeng", Name: "顺丰速运 (sf)"},
		{Code: "yuantong", Name: "圆通速递 (yt)"},
		{Code: "shentong", Name: "申通快递 (sto)"},
		{Code: "zhongtong", Name: "中通快递 (zto)"},
		{Code: "yunda", Name: "韵达快递 (yd)"},
		{Code: "jtexpress", Name: "极兔速递 (jt)"},
		{Code: "jd", Name: "京东物流 (jd)"},
		{Code: "ems", Name: "EMS (ems)"},
		{Code: "youzhengguonei", Name: "邮政国内 (yzgn)"},
		{Code: "debangwuliu", Name: "德邦物流 (dbwl)"},
		{Code: "annengwuliu", Name: "安能物流 (anneng)"},
		{Code: "huitongkuaidi", Name: "百世快递 (best)"},
		{Code: "kuayue", Name: "跨越速运 (kuayue)"},
		{Code: "ups", Name: "UPS (ups)"},
		{Code: "fedex", Name: "FedEx (fedex)"},
		{Code: "dhl", Name: "DHL (dhl)"},
	}
}
