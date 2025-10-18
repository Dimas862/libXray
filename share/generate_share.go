package share

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/xtls/xray-core/infra/conf"
)

// Convert XrayJson to share links.
// VMess will generate VMessAEAD link.
func ConvertXrayJsonToShareLinks(xrayBytes []byte) (string, error) {
	var xray *conf.Config

	err := json.Unmarshal(xrayBytes, &xray)
	if err != nil {
		return "", err
	}

	outbounds := xray.OutboundConfigs
	if len(outbounds) == 0 {
		return "", fmt.Errorf("no valid outbounds")
	}

	var links []string
	for _, outbound := range outbounds {
		link, err := shareLink(outbound)
		if err == nil {
			links = append(links, link.String())
		}
	}
	if len(links) == 0 {
		return "", fmt.Errorf("no valid outbounds")
	}
	shareText := strings.Join(links, "\n")
	return shareText, nil
}

func shareLink(proxy conf.OutboundDetourConfig) (*url.URL, error) {
	shareUrl := &url.URL{}

	switch proxy.Protocol {
	case "shadowsocks":
		err := shadowsocksLink(proxy, shareUrl)
		if err != nil {
			return nil, err
		}
	case "vmess":
		err := vmessLink(proxy, shareUrl)
		if err != nil {
			return nil, err
		}
	case "vless":
		err := vlessLink(proxy, shareUrl)
		if err != nil {
			return nil, err
		}
	case "socks":
		err := socksLink(proxy, shareUrl)
		if err != nil {
			return nil, err
		}
	case "trojan":
		err := trojanLink(proxy, shareUrl)
		if err != nil {
			return nil, err
		}
	}
	streamSettingsQuery(proxy, shareUrl)

	return shareUrl, nil
}

func shadowsocksLink(proxy conf.OutboundDetourConfig, link *url.URL) error {
	var settings *conf.ShadowsocksClientConfig
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = getOutboundName(proxy)
	link.Scheme = "ss"

	link.Host = fmt.Sprintf("%s:%d", settings.Address, settings.Port)
	password := fmt.Sprintf("%s:%s", settings.Cipher, settings.Password)
	username := base64.StdEncoding.EncodeToString([]byte(password))
	link.User = url.User(username)

	return nil
}

func vmessLink(proxy conf.OutboundDetourConfig, link *url.URL) error {
	var settings *conf.VMessOutboundConfig
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = getOutboundName(proxy)
	link.Scheme = "vmess"

	link.Host = fmt.Sprintf("%s:%d", settings.Address, settings.Port)
	link.User = url.User(settings.ID)
	if len(settings.Security) > 0 {
		link.RawQuery = addQuery(link.RawQuery, "encryption", settings.Security)
	}

	return nil
}

func vlessLink(proxy conf.OutboundDetourConfig, link *url.URL) error {
	var settings *conf.VLessOutboundConfig
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = getOutboundName(proxy)
	link.Scheme = "vless"

	link.Host = fmt.Sprintf("%s:%d", settings.Address, settings.Port)
	link.User = url.User(settings.Id)
	if len(settings.Flow) > 0 {
		link.RawQuery = addQuery(link.RawQuery, "flow", settings.Flow)
	}
	if len(settings.Encryption) > 0 {
		link.RawQuery = addQuery(link.RawQuery, "encryption", settings.Encryption)
	}

	return nil
}

func socksLink(proxy conf.OutboundDetourConfig, link *url.URL) error {
	var settings *conf.SocksClientConfig
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = getOutboundName(proxy)
	link.Scheme = "socks"

	link.Host = fmt.Sprintf("%s:%d", settings.Address, settings.Port)
	password := fmt.Sprintf("%s:%s", settings.Username, settings.Password)
	username := base64.StdEncoding.EncodeToString([]byte(password))
	link.User = url.User(username)

	return nil
}

func trojanLink(proxy conf.OutboundDetourConfig, link *url.URL) error {
	var settings *conf.TrojanClientConfig
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = getOutboundName(proxy)
	link.Scheme = "trojan"

	link.Host = fmt.Sprintf("%s:%d", settings.Address, settings.Port)
	link.User = url.User(settings.Password)

	return nil
}

func streamSettingsQuery(proxy conf.OutboundDetourConfig, link *url.URL) {
	streamSettings := proxy.StreamSetting
	if streamSettings == nil {
		return
	}
	query := link.RawQuery

	network := "raw"
	if streamSettings.Network != nil {
		network = string(*streamSettings.Network)
	}
	query = addQuery(query, "type", network)

	if len(streamSettings.Security) == 0 {
		streamSettings.Security = "none"
	}
	query = addQuery(query, "security", streamSettings.Security)

	switch network {
	case "raw":
		if streamSettings.RAWSettings == nil {
			break
		}

		headerConfig := streamSettings.RAWSettings.HeaderConfig
		if headerConfig == nil {
			break
		}
		var header *XrayRawSettingsHeader
		err := json.Unmarshal(headerConfig, &header)
		if err != nil {
			break
		}

		headerType := header.Type
		if len(headerType) > 0 {
			query = addQuery(query, "headerType", headerType)
			if header.Request == nil {
				break
			}
			path := header.Request.Path
			if len(path) > 0 {
				query = addQuery(query, "path", strings.Join(path, ","))
			}
			if header.Request.Headers == nil {
				break
			}
			host := header.Request.Headers.Host
			if len(host) > 0 {
				query = addQuery(query, "host", strings.Join(host, ","))
			}
		}
	case "kcp":
		if streamSettings.KCPSettings == nil {
			break
		}
		seed := streamSettings.KCPSettings.Seed
		if seed != nil && len(*seed) > 0 {
			query = addQuery(query, "seed", *seed)
		}

		headerConfig := streamSettings.KCPSettings.HeaderConfig
		if headerConfig == nil {
			break
		}
		var header *XrayFakeHeader
		err := json.Unmarshal(headerConfig, &header)
		if err != nil {
			break
		}

		headerType := header.Type
		if len(headerType) > 0 {
			query = addQuery(query, "headerType", headerType)
		}
	case "ws":
		if streamSettings.WSSettings == nil {
			break
		}
		path := streamSettings.WSSettings.Path
		if len(path) > 0 {
			query = addQuery(query, "path", path)
		}
		host := streamSettings.WSSettings.Host
		if len(host) > 0 {
			query = addQuery(query, "host", host)
		}
	case "grpc":
		if streamSettings.GRPCSettings == nil {
			break
		}
		mode := streamSettings.GRPCSettings.MultiMode
		if mode {
			query = addQuery(query, "mode", "multi")
		} else {
			query = addQuery(query, "mode", "gun")
		}
		serviceName := streamSettings.GRPCSettings.ServiceName
		if len(serviceName) > 0 {
			query = addQuery(query, "serviceName", serviceName)
		}
		authority := streamSettings.GRPCSettings.Authority
		if len(authority) > 0 {
			query = addQuery(query, "authority", authority)
		}
	case "httpupgrade":
		if streamSettings.HTTPUPGRADESettings == nil {
			break
		}
		host := streamSettings.HTTPUPGRADESettings.Host
		if len(host) > 0 {
			query = addQuery(query, "host", host)
		}
		path := streamSettings.HTTPUPGRADESettings.Path
		if len(path) > 0 {
			query = addQuery(query, "path", path)
		}
	case "xhttp":
		if streamSettings.XHTTPSettings == nil {
			break
		}
		host := streamSettings.XHTTPSettings.Host
		if len(host) > 0 {
			query = addQuery(query, "host", host)
		}
		path := streamSettings.XHTTPSettings.Path
		if len(path) > 0 {
			query = addQuery(query, "path", path)
		}
		mode := streamSettings.XHTTPSettings.Mode
		if len(mode) > 0 {
			query = addQuery(query, "mode", mode)
		}
		extra := streamSettings.XHTTPSettings.Extra
		if extra != nil {
			var extraConfig *conf.SplitHTTPConfig
			err := json.Unmarshal(extra, &extraConfig)
			if err == nil {
				extraBytes, err := json.Marshal(extraConfig)
				if err == nil {
					query = addQuery(query, "extra", string(extraBytes))
				}
			}
		}
	}

	switch streamSettings.Security {
	case "tls":
		if streamSettings.TLSSettings == nil {
			break
		}
		fp := streamSettings.TLSSettings.Fingerprint
		if len(fp) > 0 {
			query = addQuery(query, "fp", fp)
		}
		sni := streamSettings.TLSSettings.ServerName
		if len(sni) > 0 {
			query = addQuery(query, "sni", sni)
		}
		alpn := streamSettings.TLSSettings.ALPN
		if alpn != nil && len(*alpn) > 0 {
			query = addQuery(query, "alpn", strings.Join(*alpn, ","))
		}
		// https://github.com/XTLS/Xray-core/discussions/716
		// 4.4.3 allowInsecure
		// 没有这个字段。不安全的节点，不适合分享。
		// I don't like this field, but too many people ask for it.
		allowInsecure := streamSettings.TLSSettings.Insecure
		if allowInsecure {
			query = addQuery(query, "allowInsecure", "1")
		}
	case "reality":
		if streamSettings.REALITYSettings == nil {
			break
		}
		fp := streamSettings.REALITYSettings.Fingerprint
		if len(fp) > 0 {
			query = addQuery(query, "fp", fp)
		}
		sni := streamSettings.REALITYSettings.ServerName
		if len(sni) > 0 {
			query = addQuery(query, "sni", sni)
		}
		pbk := streamSettings.REALITYSettings.Password
		if len(pbk) > 0 {
			query = addQuery(query, "pbk", pbk)
		}
		sid := streamSettings.REALITYSettings.ShortId
		if len(sid) > 0 {
			query = addQuery(query, "sid", sid)
		}
		pqv := streamSettings.REALITYSettings.Mldsa65Verify
		if len(pqv) > 0 {
			query = addQuery(query, "pqv", pqv)
		}
		spx := streamSettings.REALITYSettings.SpiderX
		if len(spx) > 0 {
			query = addQuery(query, "spx", spx)
		}
	}

	link.RawQuery = query
}

func addQuery(query string, key string, value string) string {
	newQuery := fmt.Sprintf("%s=%s", key, url.QueryEscape(value))
	if len(query) == 0 {
		return newQuery
	} else {
		return fmt.Sprintf("%s&%s", query, newQuery)
	}
}
