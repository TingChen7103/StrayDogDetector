package constants

import (
	"fmt"
	"time"
)

var PCAP_GAMESERVER_FILTER = ""

const _GAMESERVER_NET = "210.208.80.0/24"

func init() {
	// 開放所有分流連接埠 (11000-11200, 50000-65000) 與 210.208.80.0/24 網段，避免切換分流或進副本時漏包
	// 排除目的埠 443/53/80: 本機臨時埠可能落在 portrange 內,
	// 避免把自己的 HTTPS/DNS 流量餵進解析器與 pcapng (隱私: SNI/DNS 會洩漏瀏覽紀錄)
	filter := fmt.Sprintf("tcp and (src net %s or src portrange 11000-11200 or src portrange 50000-65000) and not dst port 443 and not dst port 53 and not dst port 80", _GAMESERVER_NET)
	PCAP_GAMESERVER_FILTER = filter
}

var SERVER_START_AT = time.Now().Unix()
