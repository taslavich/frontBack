// Package pubcontract defines the versioned contract shared by cabinet and pub.
package pubcontract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/idna"
	"math"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Prefix = "twinbid:publisher:v1:"
const ReadyKey = Prefix + "{control}:ready"
const EpochKey = Prefix + "{control}:epoch"
const StateKey = Prefix + "{control}:state"

type Code struct {
	ID               string  `json:"ad_code_id"`
	UserID           string  `json:"user_id"`
	ShortCode        string  `json:"short_code"`
	Name             string  `json:"name"`
	Format           string  `json:"format"`
	TrafficType      string  `json:"traffic_type"`
	Device           string  `json:"device"`
	Active           bool    `json:"is_active"`
	Archived         bool    `json:"archived"`
	TriggerClick     int     `json:"trigger_click"`
	FrequencyMinutes float64 `json:"frequency_minutes"`
	MaxPerSession    *int    `json:"max_per_session"`
	Version          int64   `json:"version"`
	Epoch            string  `json:"epoch,omitempty"`
}

func (c Code) Validate() error {
	if strings.TrimSpace(c.Name) == "" || utf8.RuneCountInString(c.Name) > 100 || c.Format != "popunder" {
		return errors.New("name (1..100) and format=popunder required")
	}
	if c.TrafficType != "adult" && c.TrafficType != "mainstream" {
		return errors.New("traffic_type is required and must be adult or mainstream")
	}
	if c.Device != "all" && c.Device != "desktop" && c.Device != "mobile" {
		return errors.New("invalid device")
	}
	if c.TriggerClick != 1 && c.TriggerClick != 2 {
		return errors.New("trigger_click must be 1 or 2")
	}
	if math.IsNaN(c.FrequencyMinutes) || math.IsInf(c.FrequencyMinutes, 0) || c.FrequencyMinutes < 0.1 || c.FrequencyMinutes > 10080 {
		return errors.New("frequency_minutes must be 0.1..10080")
	}
	if c.MaxPerSession != nil && (*c.MaxPerSession < 1 || *c.MaxPerSession > 100000) {
		return errors.New("max_per_session must be 1..100000 or null")
	}
	return nil
}
func CodeKey(code string) string  { return Prefix + "code:" + code }
func BlockKey(host string) string { return Prefix + "block:" + SiteID(host) }
func Hash(kind string, values ...string) string {
	h := sha256.New()
	h.Write([]byte("twinbid:" + kind + ":v1\x00"))
	for _, v := range values {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(v)))
		h.Write(n[:])
		h.Write([]byte(v))
	}
	return hex.EncodeToString(h.Sum(nil))
}
func SiteID(host string) string       { return Hash("site", host) }
func ZoneID(site, code string) string { return Hash("zone", site, code) }
func Page(raw string) (string, string, error) {
	if raw == "" || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return "", "", errors.New("page_url required, whitespace forbidden")
	}
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Opaque != "" || !(u.Scheme == "https" || u.Scheme == "http") || u.Host == "" {
		return "", "", errors.New("invalid HTTP(S) page_url")
	}
	if u.Port() != "" {
		p, e := net.LookupPort("tcp", u.Port())
		if e != nil || p < 1 || p > 65535 {
			return "", "", errors.New("invalid port")
		}
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	host = strings.TrimPrefix(host, "www.")
	host, e = idna.Lookup.ToASCII(host)
	if e != nil || len(host) > 253 || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", "", errors.New("nonpublic/invalid hostname")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return "", "", errors.New("nonpublic IP")
		}
	} else {
		if !strings.Contains(host, ".") {
			return "", "", errors.New("public hostname required")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", "", errors.New("invalid hostname")
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
					return "", "", errors.New("invalid hostname")
				}
			}
		}
	}
	u.Fragment = ""
	return host, u.String(), nil
}

type Attribution struct {
	UserID      string `json:"pub_user_id"`
	CodeID      string `json:"ad_code_id"`
	ZoneID      string `json:"zone_id"`
	RequestID   string `json:"request_id"`
	SiteID      string `json:"site_id"`
	Hostname    string `json:"hostname"`
	PageURL     string `json:"page_url"`
	TrafficType string `json:"traffic_type"`
	Time        int64  `json:"time"`
}

func Sign(secret string, value any) string {
	b, _ := json.Marshal(value)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func Verify(secret string, value any, signature string) bool {
	if len(secret) < 32 {
		return false
	}
	a, e := hex.DecodeString(signature)
	if e != nil {
		return false
	}
	b, _ := hex.DecodeString(Sign(secret, value))
	return hmac.Equal(a, b)
}

// Each script touches one key, which is safe for Redis Cluster.
const PutVersionLua = `local old=redis.call('HGET',KEYS[1],'version'); local epoch=redis.call('HGET',KEYS[1],'epoch'); if epoch==ARGV[3] and old and tonumber(old)>tonumber(ARGV[1]) then return 0 end; redis.call('HSET',KEYS[1],'version',ARGV[1],'data',ARGV[2],'epoch',ARGV[3]);return 1`

// Guard keys cover every Redis Cluster slot. Losing a shard's dynamic data also
// loses its guard; atomic admission then refuses even if the control slot survived.
var slotTags = makeSlotTags()

func redisSlot(v string) int {
	var crc uint16
	for i := 0; i < len(v); i++ {
		crc ^= uint16(v[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return int(crc & 16383)
}
func makeSlotTags() [16384]string {
	var tags [16384]string
	remaining := 16384
	for n := 0; remaining > 0; n++ {
		tag := fmt.Sprintf("publisher-slot-%d", n)
		slot := redisSlot(tag)
		if tags[slot] == "" {
			tags[slot] = tag
			remaining--
		}
	}
	return tags
}
func GuardKey(zone string) string     { return GuardKeyForSlot(redisSlot(zone)) }
func GuardKeyForSlot(slot int) string { return Prefix + "{" + slotTags[slot] + "}:guard" }

const InitGuardLua = `local rid=string.match(redis.call('INFO','server'),'run_id:(%x+)');redis.call('SET',KEYS[1],ARGV[1]..':'..rid);return 1`
const CheckGuardLua = `local rid=string.match(redis.call('INFO','server'),'run_id:(%x+)');if redis.call('GET',KEYS[1])==ARGV[1]..':'..rid then return 1 end;return 0`

func SignBytes(secret string, b []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func VerifyBytes(secret string, b []byte, signature string) bool {
	if len(secret) < 32 {
		return false
	}
	a, e := hex.DecodeString(signature)
	if e != nil {
		return false
	}
	expected, _ := hex.DecodeString(SignBytes(secret, b))
	return hmac.Equal(a, expected)
}
