package constants

import (
	"fmt"
	"strings"
	"time"
)

var PCAP_GAMESERVER_FILTER = ""

// kr 기준
const _GAMESERVER_NET = "211.218.233.0/24"

var _GAMESERVER_PORT = []string{"11020", "11021", "11023"}

func init() {
	filter := fmt.Sprint("tcp and src net ", _GAMESERVER_NET)
	filter += fmt.Sprint(" and src port ( ", strings.Join(_GAMESERVER_PORT, " or "), " ) ")

	PCAP_GAMESERVER_FILTER = filter
}

var SERVER_START_AT = time.Now().Unix()
