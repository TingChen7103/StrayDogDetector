package util

import (
	"time"
)

/*
	遊戲封包送出的時間值是「伺服器所在時區的牆上時鐘」,
	必須用伺服器時區把它還原成絕對時間。

	台服為 UTC+8。韓版原始碼寫死 UTC+9 (KST),沿用到台服會讓所有時間早 1 小時,
	使「到期時間 - 封包時間」算出負的持續時間 ——
	實測 6 份團本紀錄共 299,265 筆條件事件,未修正時有 88.2% 的到期時間落在過去
	(中位數 -3569 秒);修正為 UTC+8 後中位數 +31 秒,且已知 buff 的時長與遊戲一致
	(銳利/再生/迅速 290 秒、進行曲 438 秒、戰場的序曲 374 秒)。

	若日後要支援其他區服,只需改這裡的 serverTzOffsetSec。
*/
const serverTzOffsetSec = 8 * 60 * 60 // 台服 UTC+8 (韓服為 9 * 60 * 60)

var serverTz = time.FixedZone("UTC+8", serverTzOffsetSec)

// ParseMabiTime 把封包中的 C# DateTime 值 (自西元 1 年起算的毫秒,
// 內容為伺服器當地時間) 轉為絕對時間。
func ParseMabiTime(t uint64) time.Time {
	t = t / 1000

	// c# time (0001-01-01) -> unix epoch (1970-01-01)
	t -= 62135596800

	return time.Unix(int64(t)-serverTzOffsetSec, 0).In(serverTz)
}
